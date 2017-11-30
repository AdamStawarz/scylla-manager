// Copyright (C) 2017 ScyllaDB

package repair

import (
	"context"

	"github.com/pkg/errors"
	"github.com/scylladb/mermaid/sched/runner"
	"github.com/scylladb/mermaid/uuid"
)

// ScheduleFunc schedules a repair task. A scheduled task shall be a one shot
// repair task with the given properties.
type ScheduleFunc func(ctx context.Context, clusterID uuid.UUID, props runner.TaskProperties) error

// AutoScheduler synchronises units with a cluster and schedules one shot repair
// of every unit.
type AutoScheduler struct {
	service  *Service
	schedule ScheduleFunc
}

// NewAutoScheduler creates a new AutoScheduler with a given ScheduleFunc.
func NewAutoScheduler(service *Service, f ScheduleFunc) (*AutoScheduler, error) {
	if service == nil {
		return nil, errors.New("missing service")
	}
	if f == nil {
		return nil, errors.New("missing schedule function")
	}

	return &AutoScheduler{
		service:  service,
		schedule: f,
	}, nil
}

// RunTask implements sched/runner.Runner.
func (s *AutoScheduler) RunTask(ctx context.Context, clusterID, taskID uuid.UUID, props runner.TaskProperties) error {
	if err := s.service.SyncUnits(ctx, clusterID); err != nil {
		return errors.Wrap(err, "failed to sync units")
	}

	units, err := s.service.ListUnits(ctx, clusterID, &UnitFilter{})
	if err != nil {
		return errors.Wrap(err, "failed to list units")
	}

	for _, u := range units {
		if err := s.schedule(ctx, clusterID, runner.TaskProperties{"unit_id": u.ID.String()}); err != nil {
			return errors.Wrap(err, "failed to schedule repair")
		}
	}

	return nil
}

// StopTask implements sched/runner.Runner.
func (s *AutoScheduler) StopTask(ctx context.Context, clusterID, taskID uuid.UUID, props runner.TaskProperties) error {
	return nil
}

// TaskStatus implements sched/runner.Runner.
func (s *AutoScheduler) TaskStatus(ctx context.Context, clusterID, taskID uuid.UUID, props runner.TaskProperties) (runner.Status, error) {
	return runner.StatusStopped, nil
}
