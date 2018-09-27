// Copyright (C) 2017 ScyllaDB

package sched

import (
	"sort"
	"testing"
	"time"

	"github.com/scylladb/mermaid/internal/timeutc"
	"github.com/scylladb/mermaid/sched/runner"
	"github.com/scylladb/mermaid/uuid"
)

func makeSchedule(startDate time.Time, interval, numRetries int) Schedule {
	return Schedule{
		StartDate:  startDate,
		Interval:   Duration(time.Duration(interval) * 24 * time.Hour),
		NumRetries: numRetries,
	}
}

func makeHistory(startDate time.Time, runStatus ...runner.Status) []*Run {
	runs := make([]*Run, 0, len(runStatus))
	for i, s := range runStatus {
		runs = append(runs, &Run{
			ID:        uuid.NewTime(),
			StartTime: startDate.Add(time.Duration(i) * retryTaskWait),
			Status:    s,
		})
	}
	sort.Slice(runs, func(i, j int) bool {
		return runs[i].StartTime.After(runs[j].StartTime)
	})
	return runs
}

func TestSchedNextActivation(t *testing.T) {
	t.Parallel()

	now := timeutc.Now()
	t0 := now.AddDate(0, 0, -7)
	t1 := t0.AddDate(0, 0, 2)

	table := []struct {
		S Schedule
		H []*Run
		A time.Time
	}{
		// no history, old start with retries
		{
			S: makeSchedule(t0, 7, 3),
			A: now.Add(taskStartNowSlack),
		},
		// no history, start in future > taskStartNowSlack
		{
			S: makeSchedule(now.Add(taskStartNowSlack+time.Second), 7, 3),
			A: now.Add(taskStartNowSlack + time.Second),
		},
		// no history, start in future < tastStartNowSlack
		{
			S: makeSchedule(now.Add(time.Second), 7, 3),
			A: now.Add(retryTaskWait + time.Second),
		},
		// short history 1, retry
		{
			S: makeSchedule(t0, 7, 3),
			H: makeHistory(t1, runner.StatusError),
			A: now.Add(taskStartNowSlack),
		},
		// short history 2, retry
		{
			S: makeSchedule(t0, 7, 3),
			H: makeHistory(t1, runner.StatusError, runner.StatusError),
			A: now.Add(taskStartNowSlack),
		},
		// short (recent) history, retry
		{
			S: makeSchedule(t0, 7, 3),
			H: makeHistory(now.Add(-retryTaskWait/2), runner.StatusError),
			A: now.Add(retryTaskWait / 2),
		},
		// full history, too many activations to retry, full interval
		{
			S: makeSchedule(t0, 7, 3),
			H: makeHistory(t1, runner.StatusError, runner.StatusError, runner.StatusError),
			A: t0.AddDate(0, 0, 7),
		},
		// full history with DONE, retry
		{
			S: makeSchedule(t0, 7, 3),
			H: makeHistory(t1, runner.StatusError, runner.StatusDone, runner.StatusError),
			A: now.Add(taskStartNowSlack),
		},
		// full history with STOPPED, retry
		{
			S: makeSchedule(t0, 7, 3),
			H: makeHistory(t1, runner.StatusError, runner.StatusStopped, runner.StatusError),
			A: now.Add(taskStartNowSlack),
		},
		// one shot, short history 1, retry
		{
			S: makeSchedule(t0, 0, 3),
			H: makeHistory(t1, runner.StatusError),
			A: now.Add(taskStartNowSlack),
		},
		// one shot, short history 2, retry
		{
			S: makeSchedule(t0, 0, 3),
			H: makeHistory(t1, runner.StatusError, runner.StatusError),
			A: now.Add(taskStartNowSlack),
		},
		// one shot, full history, too many activations to retry, no retry
		{
			S: makeSchedule(t0, 0, 3),
			H: makeHistory(t1, runner.StatusError, runner.StatusError, runner.StatusError),
			A: time.Time{},
		},
		// no retry, short history 1, full interval
		{
			S: makeSchedule(t0, 7, 0),
			H: makeHistory(t1, runner.StatusError),
			A: t0.AddDate(0, 0, 7),
		},
		// one shot aborted, full history, retry
		{
			S: makeSchedule(t0, 0, 3),
			H: makeHistory(t1, runner.StatusError, runner.StatusError, runner.StatusAborted),
			A: now.Add(taskStartNowSlack),
		},
		// no retry aborted, short history 1, retry
		{
			S: makeSchedule(t0, 7, 0),
			H: makeHistory(t1, runner.StatusAborted),
			A: now.Add(taskStartNowSlack),
		},
	}

	for i, test := range table {
		if activation := test.S.NextActivation(now, test.H); activation != test.A {
			t.Error(i, "expected", test.A, "got", activation)
		}
	}
}
