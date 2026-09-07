// Package filesystem exposes immutable, cached memory revisions to agent file tools.
package filesystem

import (
	"connectrpc.com/connect"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	trace2memv1 "github.com/trace2mem/trace2mem/gen/trace2mem/v1"
	"github.com/trace2mem/trace2mem/internal/domain"
	"github.com/trace2mem/trace2mem/sdk"
	"golang.org/x/sys/unix"
	"google.golang.org/protobuf/proto"
	"os"
	"path/filepath"
	"sort"
	"sync"
)

type Cache struct {
	Client     *sdk.Client
	root       string
	budgetRoot string
	manifest   *trace2memv1.GetManifestResponse
	MaxBytes   int64
	mu         sync.Mutex
	locks      map[string]*sync.Mutex
}

func New(ctx context.Context, c *sdk.Client, revision, root string, max int64) (*Cache, error) {
	res, e := c.Memory.GetManifest(ctx, connect.NewRequest(&trace2memv1.GetManifestRequest{Revision: revision}))
	if e != nil {
		return nil, e
	}
	if res.Msg.Revision == "" {
		return nil, errors.New("no published memory revision")
	}
	budgetRoot := root
	root = filepath.Join(root, domain.Hash([]byte(c.URL+"/"+res.Msg.MemoryId+"/"+res.Msg.Revision)))
	if e = os.MkdirAll(root, 0700); e != nil {
		return nil, e
	}
	cache := &Cache{Client: c, root: root, manifest: res.Msg, budgetRoot: budgetRoot, MaxBytes: max, locks: map[string]*sync.Mutex{}}
	for _, f := range res.Msg.Files {
		if !domain.ValidPath(f.Path) || len(f.Sha256) != 64 || f.Size < 0 {
			return nil, errors.New("invalid server manifest")
		}
	}
	lock, e := os.OpenFile(filepath.Join(budgetRoot, ".cache-lock"), os.O_CREATE|os.O_RDWR, 0600)
	if e != nil {
		return nil, e
	}
	defer lock.Close()
	if e = unix.Flock(int(lock.Fd()), unix.LOCK_EX); e != nil {
		return nil, e
	}
	defer unix.Flock(int(lock.Fd()), unix.LOCK_UN)
	b, _ := json.Marshal(res.Msg)
	if e = cache.makeRoom(int64(len(b)), "manifest.json"); e != nil {
		return nil, e
	}
	if e = os.MkdirAll(root, 0700); e != nil {
		return nil, e
	}
	if e = os.WriteFile(filepath.Join(root, "manifest.json"), b, 0600); e != nil {
		return nil, e
	}
	return cache, nil
}
func (c *Cache) Read(ctx context.Context, path string) ([]byte, error) {
	if !domain.ValidPath(path) {
		return nil, errors.New("invalid path")
	}
	var file *trace2memv1.File
	for _, f := range c.manifest.Files {
		if f.Path == path {
			file = f
			break
		}
	}
	if file == nil {
		return nil, os.ErrNotExist
	}
	c.mu.Lock()
	l := c.locks[file.Sha256]
	if l == nil {
		l = &sync.Mutex{}
		c.locks[file.Sha256] = l
	}
	c.mu.Unlock()
	l.Lock()
	defer l.Unlock()
	filename := filepath.Join(c.root, file.Sha256)
	b, e := os.ReadFile(filename)
	if e == nil && int64(len(b)) == file.Size && domain.Hash(b) == file.Sha256 {
		return b, nil
	}
	if e != nil && !os.IsNotExist(e) {
		return nil, e
	}
	res, e := c.Client.Memory.ReadFile(ctx, connect.NewRequest(&trace2memv1.ReadFileRequest{Revision: c.manifest.Revision, Path: path}))
	if e != nil {
		return nil, fmt.Errorf("uncached file unavailable: %w", e)
	}
	b = []byte(res.Msg.Content)
	if res.Msg.Revision != c.manifest.Revision || domain.Hash(b) != file.Sha256 || int64(len(b)) != file.Size {
		return nil, errors.New("content does not match pinned manifest")
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	lock, e := os.OpenFile(filepath.Join(c.budgetRoot, ".cache-lock"), os.O_CREATE|os.O_RDWR, 0600)
	if e != nil {
		return nil, e
	}
	defer lock.Close()
	if e = unix.Flock(int(lock.Fd()), unix.LOCK_EX); e != nil {
		return nil, e
	}
	defer unix.Flock(int(lock.Fd()), unix.LOCK_UN)
	if e = c.makeRoom(int64(len(b)), file.Sha256); e != nil {
		return nil, e
	}
	if e = os.MkdirAll(c.root, 0700); e != nil {
		return nil, e
	}
	f, e := os.CreateTemp(c.root, ".download-")
	if e != nil {
		return nil, e
	}
	defer os.Remove(f.Name())
	if _, e = f.Write(b); e != nil {
		f.Close()
		return nil, e
	}
	if e = f.Close(); e != nil {
		return nil, e
	}
	if e = os.Rename(f.Name(), filename); e != nil {
		return nil, e
	}
	return b, nil
}
func (c *Cache) makeRoom(size int64, keep string) error {
	if size > c.MaxBytes {
		return errors.New("file exceeds cache budget")
	}
	type entry struct {
		name string
		size int64
		time int64
	}
	var entries []entry
	var total int64
	err := filepath.WalkDir(c.budgetRoot, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if d.Name() == ".cache-lock" {
			return nil
		}
		i, err := d.Info()
		if err != nil {
			return err
		}
		if path != filepath.Join(c.root, keep) {
			total += i.Size()
		}
		if path != filepath.Join(c.root, keep) {
			entries = append(entries, entry{path, i.Size(), i.ModTime().UnixNano()})
		}
		return nil
	})
	if err != nil {
		return err
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].time < entries[j].time })
	for _, v := range entries {
		if total+size <= c.MaxBytes {
			break
		}
		if e := os.Remove(v.name); e != nil {
			return e
		}
		total -= v.size
		if parent := filepath.Dir(v.name); parent != c.budgetRoot {
			_ = os.Remove(parent)
		}
	}
	if total+size > c.MaxBytes {
		return errors.New("cache budget exhausted")
	}
	return nil
}
func (c *Cache) Sync(ctx context.Context, target string) error {
	return c.SyncSelected(ctx, target, nil)
}

// SyncSelected writes only selected paths; nil selects the full revision.
func (c *Cache) SyncSelected(ctx context.Context, target string, paths []string) error {
	selected := map[string]bool{}
	for _, p := range paths {
		selected[p] = true
	}
	manifest := c.Manifest()
	manifest.Files = nil

	if _, e := os.Stat(target); e == nil {
		return errors.New("sync target must not exist; choose a new snapshot directory")
	} else if !os.IsNotExist(e) {
		return e
	}
	parent := filepath.Dir(target)
	if e := os.MkdirAll(parent, 0700); e != nil {
		return e
	}
	tmp, e := os.MkdirTemp(parent, ".trace2mem-snapshot-")
	if e != nil {
		return e
	}
	defer os.RemoveAll(tmp)
	for _, f := range c.manifest.Files {
		if paths != nil && !selected[f.Path] {
			continue
		}
		manifest.Files = append(manifest.Files, f)
		b, e := c.Read(ctx, f.Path)
		if e != nil {
			return e
		}
		p := filepath.Join(tmp, filepath.FromSlash(f.Path))
		if e = os.MkdirAll(filepath.Dir(p), 0700); e != nil {
			return e
		}
		if e = os.WriteFile(p, b, 0400); e != nil {
			return e
		}
	}
	b, _ := json.MarshalIndent(manifest, "", "  ")
	if e = os.WriteFile(filepath.Join(tmp, "manifest.json"), b, 0400); e != nil {
		return e
	}
	return os.Rename(tmp, target)
}

// Manifest returns a copy so callers cannot change a cache's pinned revision.
func (c *Cache) Manifest() *trace2memv1.GetManifestResponse {
	return proto.Clone(c.manifest).(*trace2memv1.GetManifestResponse)
}
