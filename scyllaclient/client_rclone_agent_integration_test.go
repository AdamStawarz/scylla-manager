// Copyright (C) 2017 ScyllaDB

// +build all integration

package scyllaclient_test

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/scylladb/go-log"
	. "github.com/scylladb/mermaid/mermaidtest"
	"github.com/scylladb/mermaid/scyllaclient"
)

func TestRcloneS3ListDirAgentIntegration(t *testing.T) {
	testHost := ManagedClusterHost()

	client, err := scyllaclient.NewClient(scyllaclient.TestConfig(ManagedClusterHosts(), AgentAuthToken()), log.NewDevelopment())
	if err != nil {
		t.Fatal(err)
	}

	S3InitBucket(t, testBucket)

	ctx := context.Background()

	d, err := client.RcloneListDir(ctx, testHost, remotePath(""), nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(d) > 0 {
		t.Errorf("Expected bucket to be empty, got: len(files)=%d", len(d))
	}
}

func TestRcloneSkippingFilesAgentIntegration(t *testing.T) {
	config := scyllaclient.TestConfig(ManagedClusterHosts(), AgentAuthToken())
	client, err := scyllaclient.NewClient(config, log.NewDevelopment())
	if err != nil {
		t.Fatal(err)
	}

	testHost := ManagedClusterHost()

	S3InitBucket(t, testBucket)

	ctx := context.Background()

	// Create test directory with files on the test host.
	cmd := injectDataDir("rm -rf %s/tmp/copy && mkdir -p %s/tmp/copy && echo 'bar' > %s/tmp/copy/foo && echo 'foo' > %s/tmp/copy/bar")
	_, _, err = ExecOnHost(testHost, cmd)
	if err != nil {
		t.Fatal(err)
	}
	id, err := client.RcloneCopyDir(ctx, testHost, remotePath(""), "data:tmp/copy")
	if err != nil {
		t.Fatal(err)
	}

	time.Sleep(50 * time.Millisecond)

	res, err := client.RcloneTransferred(ctx, testHost, scyllaclient.RcloneDefaultGroup(id))
	if err != nil {
		t.Fatal(err)
	}
	if len(res) != 2 {
		t.Errorf("Expected two transfers, got: len(Transferred)=%d", len(res))
	}
	for _, r := range res {
		if r.Checked == true {
			t.Errorf("Expected transferred files to not be checked")
		}
		if r.Error != "" {
			t.Errorf("Expected no error got: %s, %v", r.Error, r)
		}
	}

	id, err = client.RcloneCopyDir(ctx, testHost, remotePath(""), "data:tmp/copy")
	if err != nil {
		t.Fatal(err)
	}

	time.Sleep(50 * time.Millisecond)

	res, err = client.RcloneTransferred(ctx, testHost, scyllaclient.RcloneDefaultGroup(id))
	if err != nil {
		t.Fatal(err)
	}
	if len(res) != 2 {
		t.Errorf("Expected two transfers, got: len(Transferred)=%d", len(res))
	}
	for _, r := range res {
		if r.Checked == false {
			t.Errorf("Expected transferred files to be checked")
		}
		if r.Error != "" {
			t.Errorf("Expected no error got: %s, %v", r.Error, r)
		}
	}
}

func TestRcloneStoppingTransferIntegration(t *testing.T) {
	config := scyllaclient.TestConfig(ManagedClusterHosts(), AgentAuthToken())
	client, err := scyllaclient.NewClient(config, log.NewDevelopment())
	if err != nil {
		t.Fatal(err)
	}

	testHost := ManagedClusterHost()

	S3InitBucket(t, testBucket)

	ctx := context.Background()

	// Create big enough file on the test host to keep running for long enough.
	// 1024*102400
	cmd := injectDataDir("rm -rf %s/tmp/copy && mkdir -p %s/tmp/ && dd if=/dev/zero of=%s/tmp/copy count=1024 bs=102400")
	_, _, err = ExecOnHost(testHost, cmd)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		cmd := fmt.Sprintf("rm -rf %s/tmp/copy", scyllaDataDir)
		_, _, err := ExecOnHost(testHost, cmd)
		if err != nil {
			t.Fatal(err)
		}
	}()

	time.Sleep(50 * time.Millisecond)

	id, err := client.RcloneCopyFile(ctx, testHost, remotePath("/copy"), "data:tmp/copy")
	if err != nil {
		t.Fatal(err)
	}

	res, err := client.RcloneTransferred(ctx, testHost, scyllaclient.RcloneDefaultGroup(id))
	if err != nil {
		t.Fatal(err)
	}
	if len(res) != 0 {
		t.Errorf("Expected no completed transfers, got: len(Transferred)=%d", len(res))
	}

	err = client.RcloneJobStop(ctx, testHost, id)
	if err != nil {
		t.Fatal(err)
	}

	time.Sleep(500 * time.Millisecond)

	res, err = client.RcloneTransferred(ctx, testHost, scyllaclient.RcloneDefaultGroup(id))
	if err != nil {
		t.Fatal(err)
	}
	if len(res) != 1 {
		t.Fatalf("Expected one transfer, got: len(Transferred)=%d", len(res))
	}
	if res[0].Error == "" {
		t.Fatal("Expected error but got empty")
	}
}

const scyllaDataDir = "/var/lib/scylla/data"

func injectDataDir(cmd string) string {
	return strings.ReplaceAll(cmd, "%s", scyllaDataDir)
}
