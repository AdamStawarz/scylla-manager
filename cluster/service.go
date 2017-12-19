// Copyright (C) 2017 ScyllaDB

package cluster

import (
	"bytes"
	"context"
	"net/http"
	"sort"

	"github.com/gocql/gocql"
	"github.com/pkg/errors"
	"github.com/scylladb/gocqlx"
	"github.com/scylladb/gocqlx/qb"
	"github.com/scylladb/mermaid"
	"github.com/scylladb/mermaid/log"
	"github.com/scylladb/mermaid/schema"
	"github.com/scylladb/mermaid/scyllaclient"
	"github.com/scylladb/mermaid/uuid"
)

// Change holds ID of a modified cluster and it's current value.
type Change struct {
	ID      uuid.UUID
	Current *Cluster
}

// Service manages cluster configurations.
type Service struct {
	session          *gocql.Session
	transport        http.RoundTripper
	logger           log.Logger
	onChangeListener func(ctx context.Context, c Change) error
}

// NewService creates a new service instance.
func NewService(session *gocql.Session, t http.RoundTripper, l log.Logger) (*Service, error) {
	if session == nil || session.Closed() {
		return nil, errors.New("invalid session")
	}

	return &Service{
		session:   session,
		transport: t,
		logger:    l,
	}, nil
}

// SetOnChangeListener sets a function that would be invoked when a cluster
// changes.
func (s *Service) SetOnChangeListener(f func(ctx context.Context, c Change) error) {
	s.onChangeListener = f
}

// Client returns cluster client.
func (s *Service) Client(ctx context.Context, clusterID uuid.UUID) (*scyllaclient.Client, error) {
	if s.transport == nil {
		return nil, errors.New("no transport")
	}

	c, err := s.GetClusterByID(ctx, clusterID)
	if err != nil {
		return nil, err
	}

	client, err := scyllaclient.NewClient(c.Hosts, s.transport, s.logger.Named("client"))
	if err != nil {
		return nil, err
	}

	return scyllaclient.WithConfig(client, scyllaclient.Config{
		"murmur3_partitioner_ignore_msb_bits": float64(12),
		"shard_count":                         float64(c.ShardCount),
	}), nil
}

// ListClusters returns all the clusters for a given filtering criteria.
func (s *Service) ListClusters(ctx context.Context, f *Filter) ([]*Cluster, error) {
	s.logger.Debug(ctx, "ListClusters", "filter", f)

	// validate the filter
	if err := f.Validate(); err != nil {
		return nil, mermaid.ParamError{Cause: errors.Wrap(err, "invalid filter")}
	}

	stmt, _ := qb.Select(schema.Cluster.Name).ToCql()

	q := s.session.Query(stmt).WithContext(ctx)

	var clusters []*Cluster
	if err := gocqlx.Select(&clusters, q); err != nil {
		return nil, err
	}

	sort.Slice(clusters, func(i, j int) bool {
		return bytes.Compare(clusters[i].ID.Bytes(), clusters[j].ID.Bytes()) < 0
	})

	// nothing to filter
	if f.Name == "" {
		return clusters, nil
	}

	filtered := clusters[:0]
	for _, u := range clusters {
		if u.Name == f.Name {
			filtered = append(filtered, u)
		}
	}

	return filtered, nil
}

// GetCluster returns cluster based on ID or name. If nothing was found
// mermaid.ErrNotFound is returned.
func (s *Service) GetCluster(ctx context.Context, idOrName string) (*Cluster, error) {
	if id, err := uuid.Parse(idOrName); err == nil {
		return s.GetClusterByID(ctx, id)
	}

	return s.GetClusterByName(ctx, idOrName)
}

// GetClusterByID returns repair cluster based on ID. If nothing was found
// mermaid.ErrNotFound is returned.
func (s *Service) GetClusterByID(ctx context.Context, id uuid.UUID) (*Cluster, error) {
	s.logger.Debug(ctx, "GetClusterByID", "id", id)

	stmt, names := schema.Cluster.Get()

	q := gocqlx.Query(s.session.Query(stmt).WithContext(ctx), names).BindMap(qb.M{
		"id": id,
	})
	if q.Err() != nil {
		return nil, q.Err()
	}

	var c Cluster
	if err := gocqlx.Get(&c, q.Query); err != nil {
		return nil, err
	}

	return &c, nil
}

// GetClusterByName returns repair cluster based on name. If nothing was found
// mermaid.ErrNotFound is returned.
func (s *Service) GetClusterByName(ctx context.Context, name string) (*Cluster, error) {
	s.logger.Debug(ctx, "GetClusterByName", "name", name)

	clusters, err := s.ListClusters(ctx, &Filter{Name: name})
	if err != nil {
		return nil, err
	}

	switch len(clusters) {
	case 0:
		return nil, mermaid.ErrNotFound
	case 1:
		return clusters[0], nil
	default:
		return nil, errors.Errorf("multiple clusters share the same name %q", name)
	}
}

// PutCluster upserts a cluster, cluster instance must pass Validate() checks.
// If u.ID == uuid.Nil a new one is generated.
func (s *Service) PutCluster(ctx context.Context, c *Cluster) error {
	s.logger.Debug(ctx, "PutCluster", "cluster", c)
	if c == nil {
		return errors.New("nil Cluster")
	}

	if c.ID == uuid.Nil {
		var err error
		if c.ID, err = uuid.NewRandom(); err != nil {
			return errors.Wrap(err, "couldn't generate random UUID for Cluster")
		}
	}

	// validate cluster
	if err := c.Validate(); err != nil {
		return mermaid.ParamError{Cause: errors.Wrap(err, "invalid cluster")}
	}

	// validate hosts
	if s.transport == nil {
		s.logger.Info(ctx, "validation skipped: missing transport")
	} else {
		if err := validateHosts(ctx, c.Hosts, s.hostInfo); err != nil {
			return mermaid.ParamError{Cause: errors.Wrap(err, "invalid hosts")}
		}
	}

	// check for conflicting names
	if c.Name != "" {
		conflict, err := s.GetClusterByName(ctx, c.Name)
		if err != mermaid.ErrNotFound {
			if err != nil {
				return err
			}
			if conflict.ID != c.ID {
				return mermaid.ParamError{Cause: errors.Errorf("name conflict on %q", c.Name)}
			}
		}
	}

	stmt, names := schema.Cluster.Insert()
	q := gocqlx.Query(s.session.Query(stmt).WithContext(ctx), names).BindStruct(c)

	if err := q.ExecRelease(); err != nil {
		return err
	}

	if s.onChangeListener == nil {
		return nil
	}

	return s.onChangeListener(ctx, Change{ID: c.ID, Current: c})
}

func (s *Service) hostInfo(ctx context.Context, host string) (cluster, dc string, err error) {
	client, err := scyllaclient.NewClient([]string{host}, s.transport, s.logger.Named("client"))
	if err != nil {
		return "", "", err
	}

	cluster, err = client.ClusterName(ctx)
	if err != nil {
		return
	}
	dc, err = client.Datacenter(ctx)
	if err != nil {
		return
	}

	return
}

// DeleteCluster removes cluster based on ID.
func (s *Service) DeleteCluster(ctx context.Context, id uuid.UUID) error {
	s.logger.Debug(ctx, "DeleteCluster", "id", id)

	stmt, names := schema.Cluster.Delete()

	q := gocqlx.Query(s.session.Query(stmt).WithContext(ctx), names).BindMap(qb.M{
		"id": id,
	})

	if err := q.ExecRelease(); err != nil {
		return err
	}

	if s.onChangeListener == nil {
		return nil
	}

	return s.onChangeListener(ctx, Change{ID: id})
}
