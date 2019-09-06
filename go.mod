module github.com/scylladb/mermaid

go 1.12

require (
	github.com/OneOfOne/xxhash v1.2.5 // indirect
	github.com/apcera/termtables v0.0.0-20170405184538-bcbc5dc54055
	github.com/cespare/xxhash v1.0.0
	github.com/go-chi/chi v3.3.2+incompatible
	github.com/go-chi/render v1.0.0
	github.com/go-openapi/analysis v0.19.2
	github.com/go-openapi/errors v0.19.2
	github.com/go-openapi/jsonpointer v0.19.2
	github.com/go-openapi/jsonreference v0.19.2
	github.com/go-openapi/loads v0.19.2
	github.com/go-openapi/runtime v0.19.2
	github.com/go-openapi/spec v0.19.2
	github.com/go-openapi/strfmt v0.19.0
	github.com/go-openapi/swag v0.19.2
	github.com/go-openapi/validate v0.19.2
	github.com/gobwas/glob v0.2.3
	github.com/gocql/gocql v0.0.0-20190423091413-b99afaf3b163
	github.com/golang/mock v1.2.0
	github.com/google/go-cmp v0.3.0
	github.com/hailocab/go-hostpool v0.0.0-20160125115350-e80d13ce29ed
	github.com/hashicorp/go-version v1.1.0
	github.com/pkg/errors v0.8.1
	github.com/prometheus/client_golang v1.0.0
	github.com/prometheus/client_model v0.0.0-20190129233127-fd36f4220a90
	github.com/prometheus/common v0.6.0 // indirect
	github.com/prometheus/procfs v0.0.3 // indirect
	github.com/rclone/rclone v1.49.1
	github.com/scylladb/go-log v0.0.0-20190808115121-2ceb34174b18
	github.com/scylladb/go-set v1.0.1
	github.com/scylladb/gocqlx v1.3.1
	github.com/spaolacci/murmur3 v1.1.0 // indirect
	github.com/spf13/cobra v0.0.5
	github.com/spf13/pflag v1.0.3
	github.com/stretchr/testify v1.3.1-0.20190311161405-34c6fa2dc709
	go.uber.org/atomic v1.3.2
	go.uber.org/multierr v1.1.0
	go.uber.org/zap v1.9.1
	golang.org/x/crypto v0.0.0-20190621222207-cc06ce4a13d4
	golang.org/x/sys v0.0.0-20190626221950-04f50cda93cb
	gopkg.in/yaml.v2 v2.2.2
)

replace github.com/gocql/gocql => github.com/scylladb/gocql v1.2.0
