// Copyright (C) 2017 ScyllaDB

package scyllaclienttest

import (
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/scylladb/go-log"
	"github.com/scylladb/mermaid/pkg/scyllaclient"
)

// TestHost should be used if a function in test requires host parameter.
const TestHost = "127.0.0.1"

// MakeServer creates a new server running a http.Handler.
func MakeServer(t *testing.T, h http.Handler) (host, port string, closeServer func()) {
	t.Helper()

	server := httptest.NewServer(h)

	host, port, err := net.SplitHostPort(server.Listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	closeServer = func() { server.Close() }

	return
}

// MakeClient creates a Client for testing. Typically host and port are set
// based on MakeServer result.
func MakeClient(t *testing.T, host, port string) *scyllaclient.Client {
	t.Helper()

	config := scyllaclient.DefaultConfig()
	config.Hosts = []string{host}
	config.Port = port
	config.Scheme = "http"

	client, err := scyllaclient.NewClient(config, log.NewDevelopment())
	if err != nil {
		t.Fatal(err)
	}
	return client
}

// SendFile streams a file given by name to HTTP response.
func SendFile(t *testing.T, w http.ResponseWriter, file string) {
	f, err := os.Open(file)
	if err != nil {
		t.Error(err)
		return
	}
	defer f.Close()
	if _, err := io.Copy(w, f); err != nil {
		t.Error("Copy() error", err)
	}
}
