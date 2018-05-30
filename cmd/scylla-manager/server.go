// Copyright (C) 2017 ScyllaDB

package main

import (
	"context"
	"net/http"
	"sync"
	"time"

	"github.com/gocql/gocql"
	"github.com/pkg/errors"
	log "github.com/scylladb/golog"
	"github.com/scylladb/mermaid/cluster"
	"github.com/scylladb/mermaid/repair"
	"github.com/scylladb/mermaid/restapi"
	"github.com/scylladb/mermaid/sched"
	"github.com/scylladb/mermaid/scyllaclient"
)

type server struct {
	config  *serverConfig
	session *gocql.Session
	logger  log.Logger

	provider   *scyllaclient.CachedProvider
	clusterSvc *cluster.Service
	repairSvc  *repair.Service
	schedSvc   *sched.Service

	httpServer  *http.Server
	httpsServer *http.Server
	errCh       chan error
}

func newServer(config *serverConfig, logger log.Logger) (*server, error) {
	session, err := gocqlConfig(config).CreateSession()
	if err != nil {
		return nil, errors.Wrapf(err, "database")
	}

	s := &server{
		config:  config,
		session: session,
		logger:  logger,

		errCh: make(chan error, 2),
	}

	if err := s.initServices(); err != nil {
		return nil, err
	}

	s.registerListeners()
	s.registerSchedulerRunners()

	s.initHTTPServers()

	return s, nil
}

func (s *server) initServices() error {
	var err error

	s.clusterSvc, err = cluster.NewService(s.session, s.logger.Named("cluster"))
	if err != nil {
		return errors.Wrapf(err, "cluster service")
	}
	s.provider = scyllaclient.NewCachedProvider(s.clusterSvc.Client)

	s.repairSvc, err = repair.NewService(
		s.session,
		s.config.Repair,
		s.clusterSvc.GetClusterByID,
		s.provider.Client,
		s.logger.Named("repair"),
	)
	if err != nil {
		return errors.Wrapf(err, "repair service")
	}

	s.schedSvc, err = sched.NewService(
		s.session,
		s.clusterSvc.GetClusterByID,
		s.logger.Named("scheduler"),
	)
	if err != nil {
		return errors.Wrapf(err, "scheduler service")
	}

	return nil
}

func (s *server) registerListeners() {
	s.clusterSvc.SetOnChangeListener(s.onClusterChange)
}

func (s *server) onClusterChange(ctx context.Context, c cluster.Change) error {
	s.provider.Invalidate(c.ID)
	return nil
}

func (s *server) registerSchedulerRunners() {
	s.schedSvc.SetRunner(sched.RepairTask, repair.Runner{Service: s.repairSvc})
}

func (s *server) initHTTPServers() {
	h := restapi.New(&restapi.Services{
		Cluster:   s.clusterSvc,
		Repair:    s.repairSvc,
		Scheduler: s.schedSvc,
	}, s.logger.Named("restapi"))

	if s.config.HTTP != "" {
		s.httpServer = &http.Server{Addr: s.config.HTTP, Handler: h}
	}
	if s.config.HTTPS != "" {
		s.httpsServer = &http.Server{Addr: s.config.HTTPS, Handler: h}
	}
}

func (s *server) startServices(ctx context.Context) error {
	if err := s.repairSvc.FixRunStatus(ctx); err != nil {
		return errors.Wrapf(err, "repair service")
	}

	if err := s.schedSvc.LoadTasks(ctx); err != nil {
		return errors.Wrapf(err, "schedule service")
	}

	return nil
}

func (s *server) startHTTPServers(ctx context.Context) {
	if s.httpServer != nil {
		s.logger.Info(ctx, "Starting HTTP", "address", s.httpServer.Addr)
		go func() {
			s.errCh <- s.httpServer.ListenAndServe()
		}()
	}

	if s.httpsServer != nil {
		s.logger.Info(ctx, "Starting HTTPS", "address", s.httpsServer.Addr)
		go func() {
			s.errCh <- s.httpsServer.ListenAndServeTLS(s.config.TLSCertFile, s.config.TLSKeyFile)
		}()
	}
}

func (s *server) shutdownServers(ctx context.Context, timeout time.Duration) {
	tctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	var wg sync.WaitGroup
	if s.httpServer != nil {
		s.logger.Info(ctx, "Closing HTTP")

		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := s.httpServer.Shutdown(tctx); err != nil {
				s.logger.Info(ctx, "Closing HTTP error", "error", err)
			}
		}()
	}
	if s.httpsServer != nil {
		s.logger.Info(ctx, "Closing HTTPS")

		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := s.httpsServer.Shutdown(tctx); err != nil {
				s.logger.Info(ctx, "Closing HTTPS error", "error", err)
			}
		}()
	}
	wg.Wait()
}

func (s *server) close() {
	if s.httpServer != nil {
		s.httpServer.Close()
	}
	if s.httpsServer != nil {
		s.httpsServer.Close()
	}

	s.schedSvc.Close()
	s.repairSvc.Close()

	s.session.Close()
}
