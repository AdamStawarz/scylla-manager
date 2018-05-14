// Copyright (C) 2017 ScyllaDB

package ssh

import (
	"net"

	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	"golang.org/x/crypto/ssh"
)

var (
	sshOpenStreamsCount = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "scylla_manager",
		Subsystem: "ssh",
		Name:      "open_streams_count",
		Help:      "Number of active (multiplexed) connections to Scylla node.",
	}, []string{"host"})

	sshErrorsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "scylla_manager",
		Subsystem: "ssh",
		Name:      "errors_total",
		Help:      "Total number of SSH dial errors.",
	}, []string{"host"})
)

func init() {
	prometheus.MustRegister(
		sshOpenStreamsCount,
		sshErrorsTotal,
	)
}

type proxyConn struct {
	net.Conn
	free func()
}

// Close closes the connection and frees the associated resources.
func (c proxyConn) Close() error {
	defer c.free()
	return c.Conn.Close()
}

// ProxyDialer is a dialler that allows for proxying connections over SSH.
type ProxyDialer struct {
	*Pool
	Config *ssh.ClientConfig
}

// Dial to addr HOST:PORT establishes an SSH connection to HOST and then
// proxies the connection to localhost:PORT.
func (t ProxyDialer) Dial(network, addr string) (net.Conn, error) {
	host, port, _ := net.SplitHostPort(addr)
	labels := prometheus.Labels{"host": host}

	client, err := t.Pool.Dial(network, net.JoinHostPort(host, "22"), t.Config)
	if err != nil {
		sshErrorsTotal.With(labels).Inc()
		return nil, errors.Wrap(err, "ssh: dial failed")
	}

	var (
		conn    net.Conn
		connErr error
	)
	for _, h := range []string{"localhost", host} {
		conn, connErr = client.Dial(network, net.JoinHostPort(h, port))
		if connErr == nil {
			break
		}
	}
	if connErr != nil {
		sshErrorsTotal.With(labels).Inc()
		return nil, errors.Wrap(connErr, "ssh: remote dial failed")
	}

	g := sshOpenStreamsCount.With(labels)
	g.Inc()

	return proxyConn{
		Conn: conn,
		free: func() {
			g.Dec()
			t.Pool.Release(client)
		},
	}, nil
}
