package filesystem

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReplacementAndMetadataBudget(t *testing.T) {
	root := t.TempDir()
	rev := filepath.Join(root, "revision")
	if e := os.Mkdir(rev, 0700); e != nil {
		t.Fatal(e)
	}
	if e := os.WriteFile(filepath.Join(rev, "manifest.json"), make([]byte, 80), 0600); e != nil {
		t.Fatal(e)
	}
	c := &Cache{root: rev, budgetRoot: root, MaxBytes: 100}
	if e := c.makeRoom(80, "manifest.json"); e != nil {
		t.Fatal("reopening fitting manifest failed", e)
	}
	other := filepath.Join(root, "old")
	if e := os.Mkdir(other, 0700); e != nil {
		t.Fatal(e)
	}
	if e := os.WriteFile(filepath.Join(other, "manifest.json"), make([]byte, 80), 0600); e != nil {
		t.Fatal(e)
	}
	if e := c.makeRoom(80, "manifest.json"); e != nil {
		t.Fatal(e)
	}
	if _, e := os.Stat(other); !os.IsNotExist(e) {
		t.Fatal("evicted metadata directory retained", e)
	}
}
