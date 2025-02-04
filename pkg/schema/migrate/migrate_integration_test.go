// Copyright (C) 2017 ScyllaDB

// +build all integration

package migrate

import (
	"github.com/scylladb/go-log"
	"github.com/scylladb/gocqlx/v2/migrate"
)

func init() {
	Logger = log.NewDevelopment()
	migrate.Callback = Callback
}

var _register map[nameEvent]callback

func saveRegister() {
	_register = make(map[nameEvent]callback, len(register))
	for k, v := range register {
		_register[k] = v
	}
}

func restoreRegister() {
	register = make(map[nameEvent]callback, len(_register))
	for k, v := range _register {
		register[k] = v
	}
}
