// Copyright (C) 2017 ScyllaDB

package bench

import (
	"context"
	"fmt"
	"io"
	"path"
	"runtime"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/prometheus/procfs"
	"github.com/rclone/rclone/fs"
	roperations "github.com/rclone/rclone/fs/operations"
	rsync "github.com/rclone/rclone/fs/sync"
	"github.com/scylladb/scylla-manager/pkg/rclone/operations"
	"github.com/scylladb/scylla-manager/pkg/service/backup/backupspec"
	"github.com/scylladb/scylla-manager/pkg/util/timeutc"
	"go.uber.org/multierr"
)

// Scenario contains memory stats recorded at the scenario start, end, and
// overall peak for the duration of the scenario.
type Scenario struct {
	name        string
	startedAt   time.Time
	completedAt time.Time
	startMemory memoryStats
	endMemory   memoryStats
	maxMemory   memoryStats
	err         error

	done chan struct{}
}

// StartScenario starts recording stats for a new benchmarking scenario.
func StartScenario(name string) *Scenario {
	ms := readMemoryStats()

	s := Scenario{
		name:        name,
		startedAt:   timeutc.Now(),
		startMemory: ms,
		maxMemory:   ms,
		done:        make(chan struct{}),
	}
	go s.observeMemory()

	return &s
}

func (s *Scenario) observeMemory() {
	const interval = 300 * time.Millisecond
	func() {
		t := time.NewTicker(interval)
		defer t.Stop()
		for {
			select {
			case <-t.C:
			case <-s.done:
				return
			}
			s.maxMemory.max(readMemoryStats())
		}
	}()
}

// EndScenario finishes recording stats for benchmarking scenario.
// Must be called after scenario is done to release resources.
func (s *Scenario) EndScenario() {
	s.completedAt = timeutc.Now()

	close(s.done)
	ms := readMemoryStats()
	s.endMemory = ms
	s.maxMemory.max(ms)
}

// WriteTo prints a textual report of the memory usage in the scenario.
// It must be called after EndScenario.
func (s *Scenario) WriteTo(w io.Writer) (int64, error) {
	b := &strings.Builder{}

	fmt.Fprintf(b, "Scenario:\t%s\n", path.Base(s.name))
	if s.err != nil {
		fmt.Fprintf(b, "Error:\t%s\n", s.err)
	}
	fmt.Fprintf(b, "Duration:\t%s\n", s.completedAt.Sub(s.startedAt))
	fmt.Fprintf(b, "HeapInuse:\t%d/%d/%d MiB\n", bToMb(s.startMemory.HeapInuse), bToMb(s.endMemory.HeapInuse), bToMb(s.maxMemory.HeapInuse))
	fmt.Fprintf(b, "Alloc:\t\t%d/%d/%d MiB\n", bToMb(s.startMemory.Alloc), bToMb(s.endMemory.Alloc), bToMb(s.maxMemory.Alloc))
	fmt.Fprintf(b, "TotalAlloc:\t%d/%d/%d MiB\n", bToMb(s.startMemory.TotalAlloc), bToMb(s.endMemory.TotalAlloc), bToMb(s.maxMemory.TotalAlloc))
	fmt.Fprintf(b, "Sys:\t\t%d/%d/%d MiB\n", bToMb(s.startMemory.Sys), bToMb(s.endMemory.Sys), bToMb(s.maxMemory.Sys))
	fmt.Fprintf(b, "Resident:\t%d/%d/%d MiB\n", bToMb(s.startMemory.Resident), bToMb(s.endMemory.Resident), bToMb(s.maxMemory.Resident))
	fmt.Fprintf(b, "Virtual:\t%d/%d/%d MiB\n", bToMb(s.startMemory.Virtual), bToMb(s.endMemory.Virtual), bToMb(s.maxMemory.Virtual))

	n, err := w.Write([]byte(b.String()))
	return int64(n), err
}

func bToMb(b uint64) uint64 {
	return b / 1024 / 1024
}

type memoryStats struct {
	HeapInuse  uint64
	HeapIdle   uint64
	Alloc      uint64
	TotalAlloc uint64
	Sys        uint64

	Resident uint64
	Virtual  uint64
}

func readMemoryStats() memoryStats {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	ms := memoryStats{
		HeapInuse:  m.HeapInuse,
		HeapIdle:   m.HeapIdle,
		Alloc:      m.Alloc,
		TotalAlloc: m.TotalAlloc,
		Sys:        m.Sys,
	}

	// Failure to read memory will be manifested as Resident = 0.
	if p, err := procfs.Self(); err == nil {
		stat, err := p.Stat()
		if err == nil {
			ms.Resident = uint64(stat.ResidentMemory())
			ms.Virtual = uint64(stat.VirtualMemory())
		}
	}

	return ms
}

func (ms *memoryStats) max(v memoryStats) {
	if ms.HeapInuse < v.HeapInuse {
		ms.HeapInuse = v.HeapInuse
	}
	if ms.HeapIdle < v.HeapIdle {
		ms.HeapIdle = v.HeapIdle
	}
	if ms.Alloc < v.Alloc {
		ms.Alloc = v.Alloc
	}
	if ms.TotalAlloc < v.TotalAlloc {
		ms.TotalAlloc = v.TotalAlloc
	}
	if ms.Sys < v.Sys {
		ms.Sys = v.Sys
	}
	if ms.Resident < v.Resident {
		ms.Resident = v.Resident
	}
	if ms.Virtual < v.Virtual {
		ms.Virtual = v.Virtual
	}
}

// Benchmark allows setting up and running rclone copy scenarios against
// cloud storage providers.
type Benchmark struct {
	dst fs.Fs
}

// NewBenchmark setups new benchmark object for the provided location.
func NewBenchmark(ctx context.Context, loc string) (*Benchmark, error) {
	l, err := backupspec.StripDC(loc)
	if err != nil {
		return nil, errors.Wrapf(err, loc)
	}

	f, err := fs.NewFs(ctx, l)
	if err != nil {
		if errors.Is(err, fs.ErrorNotFoundInConfigFile) {
			return nil, backupspec.ErrInvalid
		}
		return nil, errors.Wrapf(err, loc)
	}

	// Get better debug information if there are permission issues
	if err := operations.CheckPermissions(ctx, f); err != nil {
		return nil, err
	}

	return &Benchmark{dst: f}, nil
}

// StartScenario copies files from the provided dir to the benchmark location.
// It returns memory stats collected during the execution.
func (b *Benchmark) StartScenario(ctx context.Context, dir string) (*Scenario, error) {
	s := StartScenario(dir)
	cleanup, err := copyDir(ctx, dir, b.dst)
	if err != nil {
		s.err = err
	}
	s.EndScenario()

	if err := cleanup(); err != nil {
		s.err = multierr.Combine(s.err, errors.Wrap(err, "cleanup"))
	}

	return s, nil
}

func copyDir(ctx context.Context, dir string, dstFs fs.Fs) (cleanup func() error, err error) {
	const benchmarkDir = "benchmark"

	cleanup = func() error {
		return roperations.Purge(ctx, dstFs, benchmarkDir)
	}

	srcFs, err := fs.NewFs(ctx, dir)
	if err != nil {
		return cleanup, err
	}
	if err := rsync.CopyDir2(ctx, dstFs, benchmarkDir, srcFs, "", false); err != nil {
		return cleanup, err
	}

	return cleanup, err
}
