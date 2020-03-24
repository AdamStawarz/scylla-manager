// Copyright (C) 2017 ScyllaDB

package rcserver

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"time"

	"github.com/pkg/errors"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/object"
	rcops "github.com/rclone/rclone/fs/operations"
	"github.com/rclone/rclone/fs/rc"
	"github.com/rclone/rclone/fs/rc/jobs"
	"github.com/scylladb/mermaid/pkg/rclone/operations"
	"github.com/scylladb/mermaid/pkg/rclone/rcserver/internal"
	"github.com/scylladb/mermaid/pkg/util/timeutc"
	"go.uber.org/multierr"
)

// CatLimit is the maximum amount of bytes that Cat operation can output.
// Cat is used for manifest file reading. 4MB value was chosen based on
// generated manifest consisting of 2 keyspaces having 8000 tables each, where
// each table consisted of 250 sstable files. Such manifest file, after
// compression took 1.7MB of space. Doubling and rounding this value reserves
// some space for future manifest extensions.
const CatLimit = 4 * 1024 * 1024

// rcJobInfo aggregates core, transferred, and job stats into a single call.
// If jobid parameter is provided but job is not found then nil is returned for
// all three aggregated stats.
// If jobid parameter is not provided then transferred and core stats are
// returned for all groups to allow access to global transfer stats.
func rcJobInfo(ctx context.Context, in rc.Params) (out rc.Params, err error) {
	var (
		jobOut, statsOut, transOut map[string]interface{}
		jobErr, statsErr, transErr error
	)
	// Load Job status only if jobid is explicitly set.
	if jobid, err := in.GetInt64("jobid"); err == nil {
		wait, err := in.GetInt64("wait")
		if err != nil && !rc.IsErrParamNotFound(err) {
			jobErr = err
		} else if wait > 0 {
			jobErr = waitForJobFinish(ctx, jobid, wait)
		}
		if jobErr == nil {
			jobOut, jobErr = rcCalls.Get("job/status").Fn(ctx, in)
			in["group"] = fmt.Sprintf("job/%d", jobid)
		}
	}

	if jobErr == nil {
		statsOut, statsErr = rcCalls.Get("core/stats").Fn(ctx, in)
		transOut, transErr = rcCalls.Get("core/transferred").Fn(ctx, in)
	} else if errors.Is(jobErr, errJobNotFound) {
		// Job not found status will be registered as nil in "job" field.
		jobErr = nil
		fs.Errorf(nil, "Job not found")
	}

	return rc.Params{
		"job":         jobOut,
		"stats":       statsOut,
		"transferred": transOut["transferred"],
	}, multierr.Combine(jobErr, statsErr, transErr)
}

var errJobNotFound = errors.New("job not found")

func waitForJobFinish(ctx context.Context, jobid, wait int64) error {
	w := time.Second * time.Duration(wait)
	done := make(chan struct{})

	if err := jobs.OnFinish(jobid, func() {
		close(done)
	}); err != nil {
		// Returning errJobNotFound because jobs.OnFinish can fail only if job
		// is not available and it doesn't return any specific error to signal
		// that higher up the call chain.
		return errJobNotFound
	}

	timer := time.NewTimer(w)
	defer timer.Stop()

	select {
	case <-done:
		return nil
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func init() {
	rc.Add(rc.Call{
		Path:         "job/info",
		AuthRequired: true,
		Fn:           rcJobInfo,
		Title:        "Group all status calls into one",
		Help: `This takes the following parameters

- jobid - id of the job to get status of 
- wait  - seconds to wait for job operation to complete

Returns

job: job status
stats: running stats
transferred: transferred stats
`,
	})
}

// Cat a remote object.
func rcCat(ctx context.Context, in rc.Params) (out rc.Params, err error) {
	f, remote, err := rc.GetFsAndRemote(in)
	if err != nil {
		return nil, err
	}
	o, err := f.NewObject(ctx, remote)
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	w := base64.NewEncoder(base64.StdEncoding, &buf)
	if err := operations.Cat(ctx, o, w, CatLimit); err != nil {
		return nil, err
	}
	w.Close()

	out = make(rc.Params)
	out["Content"] = buf.String()
	return out, nil
}

func init() {
	rc.Add(rc.Call{
		Path:         "operations/cat",
		AuthRequired: true,
		Fn:           rcCat,
		Title:        "Concatenate any files and send them in response",
		Help: `This takes the following parameters

- fs - a remote name string eg "drive:path/to/dir"

Returns

- content - base64 encoded file content
`,
	})
}

func init() {
	rc.Add(rc.Call{
		Path:         "operations/put",
		Fn:           rcPut,
		Title:        "Save provided content as file",
		AuthRequired: true,
		Help: `This takes the following parameters:

- fs - a remote name string eg "s3:path/to/file"
- body - file content`,
	})

	// Adding it here because it is not part of the agent.json.
	// It should be removed once we are able to generate client for this call.
	internal.RcloneSupportedCalls.Add("operations/put")
}

func rcPut(ctx context.Context, in rc.Params) (out rc.Params, err error) {
	fs, remote, err := rc.GetFsAndRemote(in)
	if err != nil {
		return nil, err
	}

	body, err := in.Get("body")
	if err != nil {
		return nil, err
	}

	size, err := in.GetInt64("size")
	if err != nil {
		return nil, err
	}

	src := object.NewStaticObjectInfo(remote, timeutc.Now(), size, true, nil, nil)
	_, err = fs.Put(ctx, body.(io.Reader), src)
	return rc.Params{}, err
}

// rcCheckPermissions checks if location is available for listing, getting,
// creating, and deleting objects.
func rcCheckPermissions(ctx context.Context, in rc.Params) (out rc.Params, err error) {
	l, err := rc.GetFs(in)
	if err != nil {
		return nil, errors.Wrap(err, "init location")
	}

	if err := operations.CheckPermissions(ctx, l); err != nil {
		fs.Errorf(nil, "Location check: error=%s", err)
		return nil, err
	}

	fs.Infof(nil, "Location check done")
	return rc.Params{}, nil
}

func init() {
	rc.Add(rc.Call{
		Path:         "operations/check-permissions",
		AuthRequired: true,
		Fn:           rcCheckPermissions,
		Title:        "Checks listing, getting, creating, and deleting objects",
		Help: `This takes the following parameters

- fs - a remote name string eg "s3:repository"

`,
	})
}

// rcChunkedList supports streaming output of the listing.
func rcChunkedList(ctx context.Context, in rc.Params) (out rc.Params, err error) {
	f, remote, err := rc.GetFsAndRemote(in)
	if err != nil {
		return rc.Params{}, err
	}
	var opt rcops.ListJSONOpt
	err = in.GetStruct("opt", &opt)
	if rc.NotErrParamNotFound(err) {
		return rc.Params{}, err
	}
	v, err := in.Get("response-writer")
	if err != nil {
		return rc.Params{}, err
	}
	wf, ok := v.(writerFlusher)
	if !ok {
		panic("Invalid response writer type")
	}

	enc := newListJSONEncoder(wf, defaultListEncoderMaxItems)
	err = rcops.ListJSON(ctx, f, remote, &opt, enc.Callback)
	if err != nil {
		return enc.Result(err)
	}

	enc.Close()

	return enc.Result(nil)
}

func init() {
	rc.Calls.Get("operations/list").Fn = rcChunkedList
}

// rcCalls contains the original rc.Calls before filtering with all the added
// custom calls in this file.
var rcCalls *rc.Registry

func init() {
	rcCalls = rc.Calls
	filterRcCalls()
}

// filterRcCalls disables all default calls and whitelists only supported calls.
func filterRcCalls() {
	rc.Calls = rc.NewRegistry()

	for _, c := range rcCalls.List() {
		if internal.RcloneSupportedCalls.Has(c.Path) {
			rc.Add(*c)
		}
	}
}
