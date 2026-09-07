//go:build linux || darwin

package filesystem

import (
	"context"
	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
	"path"
	"strings"
	"syscall"
)

type root struct {
	fs.Inode
	cache *Cache
}
type file struct {
	fs.Inode
	cache *Cache
	path  string
	size  uint64
}

func (r *root) OnAdd(ctx context.Context) {
	for _, f := range r.cache.manifest.Files {
		dir := r.EmbeddedInode()
		parts := strings.Split(f.Path, "/")
		for _, part := range parts[:len(parts)-1] {
			child := dir.GetChild(part)
			if child == nil {
				child = dir.NewPersistentInode(ctx, &fs.Inode{}, fs.StableAttr{Mode: syscall.S_IFDIR})
				dir.AddChild(part, child, true)
			}
			dir = child
		}
		n := &file{cache: r.cache, path: f.Path, size: uint64(f.Size)}
		dir.AddChild(path.Base(f.Path), dir.NewPersistentInode(ctx, n, fs.StableAttr{Mode: syscall.S_IFREG}), true)
	}
}
func (f *file) Getattr(ctx context.Context, h fs.FileHandle, out *fuse.AttrOut) syscall.Errno {
	out.Mode = 0444
	out.Size = f.size
	return 0
}
func (f *file) Open(ctx context.Context, flags uint32) (fs.FileHandle, uint32, syscall.Errno) {
	if flags&(syscall.O_WRONLY|syscall.O_RDWR|syscall.O_TRUNC|syscall.O_APPEND) != 0 {
		return nil, 0, syscall.EROFS
	}
	return nil, fuse.FOPEN_KEEP_CACHE, 0
}
func (f *file) Read(ctx context.Context, h fs.FileHandle, dest []byte, off int64) (fuse.ReadResult, syscall.Errno) {
	if off < 0 {
		return nil, syscall.EINVAL
	}
	b, e := f.cache.Read(ctx, f.path)
	if e != nil {
		return nil, syscall.EIO
	}
	if off >= int64(len(b)) {
		return fuse.ReadResultData(nil), 0
	}
	end := off + int64(len(dest))
	if end > int64(len(b)) {
		end = int64(len(b))
	}
	return fuse.ReadResultData(b[off:end]), 0
}
func Mount(c *Cache, target string) (*fuse.Server, error) {
	return fs.Mount(target, &root{cache: c}, &fs.Options{MountOptions: fuse.MountOptions{Options: []string{"ro"}, Name: "brain", FsName: "brain-" + c.manifest.Revision}})
}
