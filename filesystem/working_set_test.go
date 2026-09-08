package filesystem

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	v1 "github.com/tanwargenairesearch/trace2mem/gen/trace2mem/v1"
	"github.com/tanwargenairesearch/trace2mem/internal/domain"
	"github.com/tanwargenairesearch/trace2mem/sdk"
	"google.golang.org/protobuf/proto"
	"io"
)

func cacheBackend(t testing.TB, count int) (*httptest.Server, *atomic.Int64, *atomic.Bool, *v1.GetManifestResponse) {
	t.Helper()
	manifest := &v1.GetManifestResponse{MemoryId: "user-memory", Revision: "revision", Watermark: 42}
	contents := map[string]string{}
	for i := 0; i < count; i++ {
		path := fmt.Sprintf("knowledge/page-%03d.md", i)
		body := fmt.Sprintf("# Page %d\nneedle\n%s", i, strings.Repeat("x", 1024))
		contents[path] = body
		manifest.Files = append(manifest.Files, &v1.File{Path: path, Sha256: domain.Hash([]byte(body)), Size: int64(len(body))})
	}
	calls, corrupt := &atomic.Int64{}, &atomic.Bool{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/proto")
		var reply proto.Message = manifest
		if strings.HasSuffix(r.URL.Path, "ReadFile") {
			calls.Add(1)
			var req v1.ReadFileRequest
			data, err := io.ReadAll(r.Body)
			if err != nil {
				http.Error(w, "read", 400)
				return
			}
			if err := proto.Unmarshal(data, &req); err != nil {
				http.Error(w, "decode", 400)
				return
			}
			if req.Revision != manifest.Revision {
				http.Error(w, "revision", 409)
				return
			}
			body := contents[req.Path]
			if corrupt.Load() {
				body = "interrupted"
			}
			reply = &v1.ReadFileResponse{Revision: manifest.Revision, Content: body, Sha256: domain.Hash([]byte(body))}
		}
		data, err := proto.Marshal(reply)
		if err != nil {
			t.Error(err)
			return
		}
		w.Write(data)
	}))
	t.Cleanup(srv.Close)
	return srv, calls, corrupt, manifest
}

func TestConcurrentWorkingSetAndOfflineMiss(t *testing.T) {
	srv, calls, corrupt, manifest := cacheBackend(t, 2)
	c, err := New(context.Background(), sdk.New(srv.URL, "fixture"), "revision", t.TempDir(), 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	path := manifest.Files[0].Path
	corrupt.Store(true)
	if _, err := c.Read(context.Background(), path); err == nil {
		t.Fatal("corrupt download accepted")
	}
	corrupt.Store(false)
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Go(func() {
			if _, err := c.Read(context.Background(), path); err != nil {
				t.Error(err)
			}
		})
	}
	wg.Wait()
	if calls.Load() != 2 {
		t.Fatal("downloads not coalesced", calls.Load())
	}
	srv.Close()
	before := calls.Load()
	for i := 0; i < 20; i++ {
		if _, err := c.Read(context.Background(), path); err != nil {
			t.Fatal("warm read needs backend", err)
		}
	}
	if calls.Load() != before {
		t.Fatal("warm read made a request")
	}
	if _, err := c.Read(context.Background(), manifest.Files[1].Path); err == nil || !strings.Contains(err.Error(), "uncached file unavailable") {
		t.Fatal("cold offline miss", err)
	}
}

func BenchmarkWorkingSetSearch(b *testing.B) {
	srv, _, _, manifest := cacheBackend(b, 100)
	ctx := context.Background()
	cache, err := New(ctx, sdk.New(srv.URL, "fixture"), "revision", b.TempDir(), 4<<20)
	if err != nil {
		b.Fatal(err)
	}
	directory := b.TempDir()
	for _, f := range manifest.Files {
		data, err := cache.Read(ctx, f.Path)
		if err != nil {
			b.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(directory, filepath.Base(f.Path)), data, 0600); err != nil {
			b.Fatal(err)
		}
	}
	b.Run("warm-cache", func(b *testing.B) {
		for b.Loop() {
			for _, f := range manifest.Files {
				data, err := cache.Read(ctx, f.Path)
				if err != nil || !strings.Contains(string(data), "needle") {
					b.Fatal(err)
				}
			}
		}
	})
	b.Run("directory", func(b *testing.B) {
		for b.Loop() {
			for _, f := range manifest.Files {
				data, err := os.ReadFile(filepath.Join(directory, filepath.Base(f.Path)))
				if err != nil || !strings.Contains(string(data), "needle") {
					b.Fatal(err)
				}
			}
		}
	})
	b.Run("cold-cache", func(b *testing.B) {
		for b.Loop() {
			c, err := New(ctx, sdk.New(srv.URL, "fixture"), "revision", b.TempDir(), 4<<20)
			if err != nil {
				b.Fatal(err)
			}
			for _, f := range manifest.Files {
				data, err := c.Read(ctx, f.Path)
				if err != nil || !strings.Contains(string(data), "needle") {
					b.Fatal(err)
				}
			}
		}
	})
}
