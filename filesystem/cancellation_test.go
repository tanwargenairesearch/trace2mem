package filesystem

import (
	"context"
	"errors"
	"github.com/mohit-lendmind/trace2mem/sdk"
	"golang.org/x/sys/unix"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCanceledFollowerDoesNotWaitForDownload(t *testing.T) {
	backend, _, _, manifest := cacheBackend(t, 1)
	started, release := make(chan struct{}), make(chan struct{})
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "ReadFile") {
			close(started)
			select {
			case <-release:
			case <-r.Context().Done():
				return
			}
		}
		backend.Config.Handler.ServeHTTP(w, r)
	}))
	defer func() { close(release); proxy.Close() }()
	c, err := New(context.Background(), sdk.New(proxy.URL, "fixture"), "revision", t.TempDir(), 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	leader, cancelLeader := context.WithCancel(context.Background())
	defer cancelLeader()
	done := make(chan error, 1)
	go func() { _, err := c.Read(leader, manifest.Files[0].Path); done <- err }()
	select {
	case <-started:
	case err := <-done:
		t.Fatalf("leader failed before download: %v", err)
	case <-time.After(time.Second):
		t.Fatal("download did not start")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	follower := make(chan error, 1)
	go func() { _, err := c.Read(ctx, manifest.Files[0].Path); follower <- err }()
	select {
	case err := <-follower:
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("canceled follower waits for leader")
	}
	cancelLeader()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("leader did not cancel")
	}
}

func TestFileLockWaitIsCancellable(t *testing.T) {
	path := filepath.Join(t.TempDir(), "lock")
	a, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	b, err := os.OpenFile(path, os.O_RDWR, 0600)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	if err = unix.Flock(int(a.Fd()), unix.LOCK_EX); err != nil {
		t.Fatal(err)
	}
	defer unix.Flock(int(a.Fd()), unix.LOCK_UN)
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- lockFile(ctx, b) }()
	select {
	case err = <-done:
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("file lock ignores cancellation")
	}
}
