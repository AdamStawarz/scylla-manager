// Copyright (C) 2017 ScyllaDB

package repair

import (
	"context"
	"math/rand"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/scylladb/go-log"
	"github.com/scylladb/go-set/strset"
)

// TODO add docs to types

type hostPriority map[string]int

func (hp hostPriority) PickHost(replicas []string) string {
	for p := 0; p < len(hp); p++ {
		for _, r := range replicas {
			if hp[r] == p {
				return r
			}
		}
	}
	return replicas[0]
}

type hostRangesLimit map[string]int

type job struct {
	Host   string
	Ranges []*tableTokenRange
}

type jobResult struct {
	job
	Err error
}

type generator struct {
	gracefulShutdownTimeout time.Duration
	logger                  log.Logger

	replicas     map[uint64][]string
	ranges       map[uint64][]*tableTokenRange
	hostCount    int
	hostPriority hostPriority
	smallTables  *strset.Set
	progress     progressManager

	intensity        float64
	intensityHandler *intensityHandler

	keys       []uint64
	pos        int
	busy       *strset.Set
	next       chan job
	nextClosed bool
	result     chan jobResult

	count   int
	success int
	failed  int
}

func newGenerator(ih *intensityHandler, gracefulShutdownTimeout time.Duration, logger log.Logger, manager progressManager) *generator {
	g := &generator{
		gracefulShutdownTimeout: gracefulShutdownTimeout,
		logger:                  logger,
		replicas:                make(map[uint64][]string),
		ranges:                  make(map[uint64][]*tableTokenRange),
		smallTables:             strset.New(),
		progress:                manager,
	}

	// Check if intensity channel has desired intensity value
	select {
	case intensity := <-ih.c:
		g.intensity = intensity
	default:
	}

	g.intensityHandler = ih

	return g
}

func (g *generator) Add(ranges []*tableTokenRange) {
	for _, ttr := range ranges {
		g.add(ttr)
	}
}

func (g *generator) add(ttr *tableTokenRange) {
	hash := ttr.ReplicaHash()
	g.replicas[hash] = ttr.Replicas
	g.ranges[hash] = append(g.ranges[hash], ttr)
}

func (g *generator) Hosts() *strset.Set {
	all := strset.New()
	for _, v := range g.replicas {
		all.Add(v...)
	}
	return all
}

func (g *generator) SetHostPriority(hp hostPriority) {
	g.hostPriority = hp
}

func (g *generator) Init(ctx context.Context, workerCount int) error {
	if len(g.replicas) == 0 {
		panic("cannot init generator, no ranges")
	}
	g.keys = make([]uint64, 0, len(g.replicas))
	for k := range g.replicas {
		g.keys = append(g.keys, k)
	}
	g.hostCount = g.Hosts().Size()
	g.pos = rand.Intn(len(g.keys))
	g.busy = strset.New()
	g.next = make(chan job, 2*workerCount)
	g.result = make(chan jobResult)

	var trs []*tableTokenRange
	for _, ttrs := range g.ranges {
		trs = append(trs, ttrs...)
	}
	if err := g.progress.Init(ctx, trs); err != nil {
		return err
	}

	// Remove repaired ranges from the pool of available ones to avoid their
	// scheduling.
	for k := range g.ranges {
		ttrs := g.ranges[k][:0]
		for i := range g.ranges[k] {
			if g.progress.CheckRepaired(g.ranges[k][i]) {
				continue
			}
			ttrs = append(ttrs, g.ranges[k][i])
		}
		g.ranges[k] = ttrs
		g.count += len(ttrs)
	}

	g.fillNext()

	return nil
}

func (g *generator) Next() <-chan job {
	return g.next
}

func (g *generator) Result() chan<- jobResult {
	return g.result
}

func (g *generator) Run(ctx context.Context) (err error) {
	g.logger.Info(ctx, "Start repair")

	//TODO: progress and state registration
	lastPercent := -1

	done := ctx.Done()
	stop := make(chan struct{})
loop:
	for {
		select {
		case <-stop:
			break loop
		case <-done:
			g.logger.Info(ctx, "Graceful repair shutdown", "timeout", g.gracefulShutdownTimeout)
			// Stop workers by closing next channel
			g.closeNext()

			done = nil
			time.AfterFunc(g.gracefulShutdownTimeout, func() {
				close(stop)
			})
			err = ctx.Err()
		case intensity := <-g.intensityHandler.c:
			g.logger.Info(ctx, "Changing repair intensity", "from", g.intensity, "to", intensity)
			g.intensity = intensity
		case r := <-g.result:
			// TODO handling penalties
			lastPercent = g.processResult(ctx, r, lastPercent)

			g.fillNext()

			if done := g.busy.IsEmpty(); done {
				g.logger.Info(ctx, "Done repair")

				g.closeNext()
				break loop
			}
		}
	}

	if g.failed > 0 {
		return errors.Errorf("%d token ranges out of %d failed to repair", g.failed, g.count)
	}
	return err
}

func (g *generator) processResult(ctx context.Context, r jobResult, lastPercent int) int {
	if err := g.progress.Update(ctx, r); err != nil {
		g.logger.Error(ctx, "Failed to update progress", "error", err)
	}

	if r.Err != nil {
		g.failed += len(r.Ranges)
		g.logger.Info(ctx, "Repair failed", "error", r.Err)
	} else {
		g.success += len(r.Ranges)
	}

	if percent := 100 * (g.success + g.failed) / g.count; percent > lastPercent {
		g.logger.Info(ctx, "Progress", "percent", percent, "count", g.count, "success", g.success, "failed", g.failed)
		lastPercent = percent
	}

	g.unblockReplicas(r.Ranges[0])

	return lastPercent
}

func (g *generator) unblockReplicas(ttr *tableTokenRange) {
	g.busy.Remove(ttr.Replicas...)
}

func (g *generator) closeNext() {
	if !g.nextClosed {
		close(g.next)
		g.nextClosed = true
	}
}

func (g *generator) fillNext() {
	if g.nextClosed {
		return
	}
	for {
		hash := g.pickReplicas()
		if hash == 0 {
			return
		}

		host := g.pickHost(hash)
		rangesLimit := g.rangesLimit(host)

		select {
		case g.next <- job{
			Host:   host,
			Ranges: g.pickRanges(hash, rangesLimit),
		}:
		default:
			panic("next buffer full")
		}
	}
}

// pickReplicas blocks replicas and returns hash, if no replicas can be found
// then 0 is returned.
func (g *generator) pickReplicas() uint64 {
	var (
		stop = g.pos
		pos  = g.pos
	)

	for {
		pos = (pos + 1) % len(g.keys)
		hash := g.keys[pos]

		if len(g.ranges[hash]) > 0 {
			replicas := g.replicas[hash]
			if g.canScheduleRepair(replicas) {
				g.busy.Add(replicas...)
				g.pos = pos
				return hash
			}
		}

		if pos == stop {
			return 0
		}
	}
}

func (g *generator) canScheduleRepair(hosts []string) bool {
	// Always make some progress
	if g.busy.IsEmpty() {
		return true
	}
	// Repair intensity might limit active hosts.
	// Check if adding these hosts to busy pool will not reach the limit.
	if len(hosts)+g.busy.Size() <= g.activeHostLimit() {
		// If those hosts aren't busy at the moment
		if !g.busy.HasAny(hosts...) {
			return true
		}
	}
	return false
}

func (g *generator) activeHostLimit() int {
	activeHostsLimit := g.hostCount
	if g.intensity > 0 && g.intensity < 1 {
		activeHostsLimit = int(g.intensity * float64(g.hostCount))
	}
	return activeHostsLimit
}

func (g *generator) pickRanges(hash uint64, limit int) []*tableTokenRange {
	ranges := g.ranges[hash]

	// Speedup repair of system and small tables by repairing all ranges together.
	if strings.HasPrefix(ranges[0].Keyspace, "system") || g.smallTable(ranges[0].Keyspace, ranges[0].Table) {
		limit = len(ranges)
	}

	var i int
	for i = 0; i < limit; i++ {
		if len(ranges) <= i {
			break
		}
		if i > 0 {
			if ranges[i-1].Keyspace != ranges[i].Keyspace || ranges[i-1].Table != ranges[i].Table {
				break
			}
		}
	}

	g.ranges[hash] = ranges[i:]
	return ranges[0:i]
}

func (g *generator) markSmallTable(keyspace, table string) {
	g.smallTables.Add(keyspace + "." + table)
}

func (g *generator) smallTable(keyspace, table string) bool {
	return g.smallTables.Has(keyspace + "." + table)
}

func (g *generator) rangesLimit(host string) int {
	limit := int(g.intensityHandler.Intensity(host))
	if limit == 0 {
		limit = 1
	}
	return limit
}

func (g *generator) pickHost(hash uint64) string {
	return g.hostPriority.PickHost(g.replicas[hash])
}
