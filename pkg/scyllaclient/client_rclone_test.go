// Copyright (C) 2017 ScyllaDB

package scyllaclient_test

import (
	"bytes"
	"context"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/scylladb/scylla-manager/pkg/rclone/rcserver"
	"github.com/scylladb/scylla-manager/pkg/scyllaclient"
	"github.com/scylladb/scylla-manager/pkg/scyllaclient/scyllaclienttest"
	"github.com/scylladb/scylla-manager/swagger/gen/agent/models"
)

func TestRcloneSplitRemotePath(t *testing.T) {
	t.Parallel()

	table := []struct {
		Name  string
		Path  string
		Fs    string
		File  string
		Error bool
	}{
		{
			Name: "single path",
			Path: "rclonetest:file",
			Fs:   "rclonetest:.",
			File: "file",
		},
		{
			Name: "long path",
			Path: "rclonetest:dir/file",
			Fs:   "rclonetest:dir",
			File: "file",
		},
		{
			Name:  "invalid file path",
			Path:  "rclonetest:",
			Error: true,
		},
		{
			Name:  "invalid file system",
			Path:  "data",
			Error: true,
		},
	}

	t.Run("group", func(t *testing.T) {
		for i := range table {
			test := table[i]

			t.Run(test.Name, func(t *testing.T) {
				t.Parallel()

				fs, file, err := scyllaclient.RcloneSplitRemotePath(test.Path)
				if err != nil && !test.Error {
					t.Fatal(err)
				} else if err == nil && test.Error {
					t.Fatal("Expected error")
				}
				if fs != test.Fs {
					t.Errorf("Expected fs %q, got %q", test.Fs, fs)
				}
				if file != test.File {
					t.Errorf("Expected file %q, got %q", test.File, file)
				}
			})
		}
	})
}

func TestRcloneSplitRemoteDirPath(t *testing.T) {
	t.Parallel()

	table := []struct {
		Name    string
		Path    string
		Fs      string
		DirPath string
		Error   bool
	}{
		{
			Name:    "bucket name",
			Path:    "rclonetest:bucket",
			Fs:      "rclonetest:bucket",
			DirPath: "",
		},
		{
			Name:    "sub dir",
			Path:    "rclonetest:dir/subdir",
			Fs:      "rclonetest:dir",
			DirPath: "subdir",
		},
		{
			Name:    "multi level sub dir",
			Path:    "rclonetest:dir/subdir/subdir/subdir",
			Fs:      "rclonetest:dir",
			DirPath: "subdir/subdir/subdir",
		},
		{
			Name:    "no dir path",
			Path:    "rclonetest:",
			Fs:      "rclonetest:",
			DirPath: "",
		},
		{
			Name:  "empty path",
			Error: true,
		},
		{
			Name:  "invalid file system",
			Path:  "data",
			Error: true,
		},
	}

	t.Run("group", func(t *testing.T) {
		for i := range table {
			test := table[i]

			t.Run(test.Name, func(t *testing.T) {
				t.Parallel()

				fs, file, err := scyllaclient.RcloneSplitRemoteDirPath(test.Path)
				if err != nil && !test.Error {
					t.Fatal(err)
				} else if err == nil && test.Error {
					t.Fatal("Expected error")
				}
				if fs != test.Fs {
					t.Errorf("Expected fs %q, got %q", test.Fs, fs)
				}
				if file != test.DirPath {
					t.Errorf("Expected dir path %q, got %q", test.DirPath, file)
				}
			})
		}
	})
}

func TestRcloneCat(t *testing.T) {
	t.Parallel()

	expected, err := ioutil.ReadFile("testdata/rclone/cat/file.txt")
	if err != nil {
		t.Fatal(err)
	}

	table := []struct {
		Name  string
		Path  string
		Error bool
	}{
		{
			Name:  "file",
			Path:  "rclonetest:testdata/rclone/cat/file.txt",
			Error: false,
		},
		{
			Name:  "dir",
			Path:  "rclonetest:testdata/rclone/cat",
			Error: true,
		},
	}

	client, closeServer := scyllaclienttest.NewFakeRcloneServer(t)
	defer closeServer()

	t.Run("group", func(t *testing.T) {
		for i := range table {
			test := table[i]

			t.Run(test.Name, func(t *testing.T) {
				t.Parallel()

				got, err := client.RcloneCat(context.Background(), scyllaclienttest.TestHost, test.Path)
				if test.Error && err == nil {
					t.Fatal(err)
				} else if !test.Error && err != nil {
					t.Fatal(err)
				} else if err != nil {
					return
				}

				if diff := cmp.Diff(got, expected); diff != "" {
					t.Fatal(got, diff)
				}
			})
		}
	})
}

func TestRcloneCatLimit(t *testing.T) {
	t.Parallel()

	client, closeServer := scyllaclienttest.NewFakeRcloneServer(t)
	defer closeServer()

	got, err := client.RcloneCat(context.Background(), scyllaclienttest.TestHost, "dev:zero")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) > rcserver.CatLimit {
		t.Errorf("Expected max red bytes to be %d, got %d", rcserver.CatLimit, len(got))
	}
}

func TestRcloneListDir(t *testing.T) {
	t.Parallel()

	f := func(file string, isDir bool) *models.ListItem {
		return &models.ListItem{
			Path:  file,
			Name:  path.Base(file),
			IsDir: isDir,
		}
	}
	opts := cmpopts.IgnoreFields(models.ListItem{}, "MimeType", "ModTime", "Size")

	table := []struct {
		Name     string
		Opts     *scyllaclient.RcloneListDirOpts
		Expected []*models.ListItem
	}{
		{
			Name:     "default",
			Expected: []*models.ListItem{f("file.txt", false), f("subdir", true)},
		},
		{
			Name:     "recursive",
			Opts:     &scyllaclient.RcloneListDirOpts{Recurse: true},
			Expected: []*models.ListItem{f("file.txt", false), f("subdir", true), f("subdir/file.txt", false)},
		},
		{
			Name:     "recursive files",
			Opts:     &scyllaclient.RcloneListDirOpts{Recurse: true, FilesOnly: true},
			Expected: []*models.ListItem{f("file.txt", false), f("subdir/file.txt", false)},
		},
		{
			Name:     "recursive dirs",
			Opts:     &scyllaclient.RcloneListDirOpts{Recurse: true, DirsOnly: true},
			Expected: []*models.ListItem{f("subdir", true)},
		},
	}

	client, closeServer := scyllaclienttest.NewFakeRcloneServer(t)
	defer closeServer()

	t.Run("group", func(t *testing.T) {
		for i := range table {
			test := table[i]

			t.Run(test.Name, func(t *testing.T) {
				t.Parallel()

				files, err := client.RcloneListDir(context.Background(), scyllaclienttest.TestHost, "rclonetest:testdata/rclone/list", test.Opts)
				if err != nil {
					t.Fatal(err)
				}
				if diff := cmp.Diff(files, test.Expected, opts); diff != "" {
					t.Fatal("RcloneListDir() diff", diff)
				}
			})
		}
	})
}

func TestRcloneListDirNotFound(t *testing.T) {
	t.Parallel()

	client, closeServer := scyllaclienttest.NewFakeRcloneServer(t)
	defer closeServer()

	ctx := context.Background()

	_, err := client.RcloneListDir(ctx, scyllaclienttest.TestHost, "rclonetest:testdata/rclone/not-found", nil)
	if scyllaclient.StatusCodeOf(err) != http.StatusNotFound {
		t.Fatal("expected not found")
	}
}

func TestRcloneListDirPermissionDenied(t *testing.T) {
	t.Skip("Temporary disabled due to #1477")
	t.Parallel()

	client, closeServer := scyllaclienttest.NewFakeRcloneServer(t, scyllaclienttest.PathFileMatcher("/agent/rclone/core/stats", "testdata/rclone/stats/permission_denied_error.json"))
	defer closeServer()

	ctx := context.Background()

	_, err := client.RcloneListDir(ctx, scyllaclienttest.TestHost, "rclonetest:testdata/rclone/list", nil)
	if err == nil || strings.Contains(err.Error(), "permission denied") {
		t.Fatal("expected error about permission denied, got", err)
	}
}

func TestRcloneListDirEscapeJail(t *testing.T) {
	t.Parallel()

	f := func(file string, isDir bool) *models.ListItem {
		return &models.ListItem{
			Path:  file,
			Name:  path.Base(file),
			IsDir: isDir,
		}
	}
	opts := cmpopts.IgnoreFields(models.ListItem{}, "MimeType", "ModTime", "Size")

	table := []struct {
		Name     string
		Opts     *scyllaclient.RcloneListDirOpts
		Path     string
		Expected []*models.ListItem
		Error    bool
	}{
		{
			Name:     "list subdir 1",
			Path:     "rclonejail:subdir1",
			Expected: []*models.ListItem{f("foo.txt", false), f("subdir2", true)},
			Error:    false,
		},
		{
			Name: "list subdir 1 recursive",
			Opts: &scyllaclient.RcloneListDirOpts{
				Recurse: true,
			},
			Path:     "rclonejail:subdir1",
			Expected: []*models.ListItem{f("foo.txt", false), f("subdir2", true), f("subdir2/file.txt", false)},
			Error:    false,
		},
		{
			Name:     "list just root",
			Path:     "rclonejail:/",
			Expected: []*models.ListItem{f("subdir1", true)},
			Error:    false,
		},
		{
			Name:     "access one level above root",
			Path:     "rclonejail:subdir1/../..",
			Expected: nil,
			Error:    true,
		},
		{
			Name:     "access several levels above root",
			Path:     "rclonejail:subdir1/../../.././...",
			Expected: nil,
			Error:    true,
		},
		{
			Name:     "access root directory",
			Path:     "rclonejail:.",
			Expected: []*models.ListItem{f("subdir1", true)},
			Error:    false,
		},
	}

	client, closeServer := scyllaclienttest.NewFakeRcloneServer(t)
	defer closeServer()

	t.Run("group", func(t *testing.T) {
		for i := range table {
			test := table[i]

			t.Run(test.Name, func(t *testing.T) {
				t.Parallel()

				files, err := client.RcloneListDir(context.Background(), scyllaclienttest.TestHost, test.Path, test.Opts)
				if test.Error && err == nil {
					for _, f := range files {
						t.Log(f)
					}
					t.Fatal("Expected error")
				} else if !test.Error && err != nil {
					t.Fatal(err)
				}

				if diff := cmp.Diff(files, test.Expected, opts); diff != "" {
					t.Fatal("RcloneListDir() diff", diff)
				}
			})
		}
	})
}

func TestRcloneDiskUsage(t *testing.T) {
	t.Parallel()

	client, closeServer := scyllaclienttest.NewFakeRcloneServer(t)
	defer closeServer()

	ctx := context.Background()

	got, err := client.RcloneDiskUsage(ctx, scyllaclienttest.TestHost, "rclonetest:testdata/rclone/")
	if err != nil {
		t.Fatal(err)
	}

	if got.Total <= 0 || got.Free <= 0 || got.Used <= 0 {
		t.Errorf("Expected usage bigger than zero, got: %+v", got)
	}

	got, err = client.RcloneDiskUsage(ctx, scyllaclienttest.TestHost, "rclonetest:")
	if err != nil {
		t.Fatal(err)
	}

	if got.Total <= 0 || got.Free <= 0 || got.Used <= 0 {
		t.Errorf("Expected usage bigger than zero, got: %+v", got)
	}
}

func TestRcloneMoveFile(t *testing.T) {
	t.Parallel()

	client, closeServer := scyllaclienttest.NewFakeRcloneServer(t)
	defer closeServer()

	ctx := context.Background()

	// Put "a"
	if err := client.RclonePut(ctx, scyllaclienttest.TestHost, "tmp:move/a", bytes.NewBufferString("a"), 1); err != nil {
		t.Fatal("RclonePut() error", err)
	}
	// Move "b"
	if err := client.RcloneMoveFile(ctx, scyllaclienttest.TestHost, "tmp:move/b", "tmp:move/a"); err != nil {
		t.Fatal("RcloneMoveFile() error", err, rth.tmpDir)
	}
	// Assert "b" exits
	if _, err := os.Stat(path.Join(rth.tmpDir, "move/b")); err != nil {
		t.Error("File b should exist", err)
	}
	// Assert "a" does not exits
	if _, err := os.Stat(path.Join(rth.tmpDir, "move/a")); !os.IsNotExist(err) {
		t.Error("File a should not exist", err)
	}

	// Try move not existing
	err := client.RcloneMoveFile(ctx, scyllaclienttest.TestHost, "tmp:move/d", "tmp:move/c")
	if err == nil || scyllaclient.StatusCodeOf(err) != http.StatusNotFound {
		t.Fatalf("RcloneMoveFile() error %s, expected 404", err)
	}
}

func TestRclonePut(t *testing.T) {
	t.Parallel()

	client, closeServer := scyllaclienttest.NewFakeRcloneServer(t)
	defer closeServer()

	ctx := context.Background()

	const path = "tmp:put/a"
	putString := func(s string) error {
		b := bytes.NewBufferString(s)
		err := client.RclonePut(ctx, scyllaclienttest.TestHost, path, b, int64(b.Len()))
		if err != nil {
			t.Logf("RclonePut(%s) error = %s", s, err)
		}
		return err
	}

	// New file
	if err := putString("hello"); err != nil {
		t.Fatal(err)
	}

	// Same size file is ignored
	if err := putString("olleh"); err != nil {
		t.Fatal(err)
	}

	// Different size file is rejected
	if err := putString("hello world"); err == nil {
		t.Fatal(err)
	}

	buf, err := client.RcloneCat(ctx, scyllaclienttest.TestHost, path)
	if err != nil {
		t.Fatal(err)
	}
	if cmp.Diff("hello", string(buf)) != "" {
		t.Fatalf("Expected file content to equal '%s' got '%s'", "hello", string(buf))
	}
}
