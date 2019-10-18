// Copyright (C) 2017 ScyllaDB

package mermaid

import "time"

// PrometheusScrapeInterval specifies how often internal metrics
// should be aggregated and exported.
var PrometheusScrapeInterval = 5 * time.Second
