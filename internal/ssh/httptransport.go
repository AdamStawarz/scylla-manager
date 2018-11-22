// Copyright (C) 2017 ScyllaDB

package ssh

import (
	"net/http"
	"time"

	"github.com/pkg/errors"
	"golang.org/x/crypto/ssh"
)

// NewTransport is a convenience function that returns a modified version of
// http.Transport that uses ProxyDialer.
func NewTransport(c *ssh.ClientConfig) *http.Transport {
	return &http.Transport{
		DialContext:           ProxyDialer{Pool: DefaultPool, Config: c}.DialContext,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}
}

// NewProductionTransport returns Transport for NewProductionClientConfig.
func NewProductionTransport(c Config) (*http.Transport, error) {
	cfg, err := NewProductionClientConfig(c)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create SSH client c")
	}

	return NewTransport(cfg), nil
}

// NewDevelopmentTransport returns Transport for NewDevelopmentClientConfig.
func NewDevelopmentTransport() *http.Transport {
	return NewTransport(NewDevelopmentClientConfig())
}