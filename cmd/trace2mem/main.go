package main

import (
	"bytes"
	"connectrpc.com/connect"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"github.com/trace2mem/trace2mem/filesystem"
	trace2memv1 "github.com/trace2mem/trace2mem/gen/trace2mem/v1"
	"github.com/trace2mem/trace2mem/internal/config"
	"github.com/trace2mem/trace2mem/sdk"
	"io"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

func output(v any) error {
	b, e := json.MarshalIndent(v, "", "  ")
	if e != nil {
		return e
	}
	fmt.Println(string(b))
	return nil
}
func main() {
	if e := run(); e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(1)
	}
}
func run() error {
	if len(os.Args) < 2 {
		return errors.New("usage: trace2mem setup|configure|doctor|import|status|compile|search|context|sync|mount|export|forget [flags]")
	}
	command := os.Args[1]
	flags := flag.NewFlagSet(command, flag.ContinueOnError)
	url := flags.String("url", env("TRACE2MEM_URL", "http://localhost:8787"), "server URL")
	token := flags.String("token", os.Getenv("TRACE2MEM_TOKEN"), "access token (prefer TRACE2MEM_TOKEN)")
	file := flags.String("file", "", "input JSONL file")
	target := flags.String("target", "", "snapshot or mount directory")
	revision := flags.String("revision", "", "published revision")
	cache := flags.String("cache", filepath.Join(os.TempDir(), "trace2mem-cache"), "local cache directory")
	query := flags.String("query", "", "retrieval query")
	event := flags.String("event", "", "event ID")
	max := flags.Int64("cache-bytes", 256<<20, "cache byte budget")
	git := flags.Bool("git", false, "initialize a Git repository for an exported snapshot")
	if e := flags.Parse(os.Args[2:]); e != nil {
		return e
	}
	if command == "setup" {
		return setup()
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	c := sdk.New(*url, *token)
	manage := func(route string, body any) (any, error) {
		var reader io.Reader
		if body != nil {
			b, e := json.Marshal(body)
			if e != nil {
				return nil, e
			}
			reader = bytes.NewReader(b)
		}
		method := "GET"
		if body != nil {
			method = "POST"
		}
		r, e := http.NewRequestWithContext(ctx, method, *url+"/api/"+route, reader)
		if e != nil {
			return nil, e
		}
		r.Header.Set("Authorization", "Bearer "+*token)
		r.Header.Set("Content-Type", "application/json")
		res, e := c.HTTP.Do(r)
		if e != nil {
			return nil, e
		}
		defer res.Body.Close()
		if res.StatusCode/100 != 2 {
			b, _ := io.ReadAll(io.LimitReader(res.Body, 4096))
			return nil, fmt.Errorf("HTTP %d: %s", res.StatusCode, b)
		}
		var v any
		e = json.NewDecoder(res.Body).Decode(&v)
		return v, e
	}
	switch command {
	case "doctor":
		client := &http.Client{Timeout: 5 * time.Second}
		r, e := client.Get(*url + "/readyz")
		if e != nil {
			return e
		}
		defer r.Body.Close()
		if r.StatusCode != 200 {
			return fmt.Errorf("server readiness: HTTP %d", r.StatusCode)
		}
		return output(map[string]any{"server_status": r.StatusCode, "fuse_device": exists("/dev/fuse")})
	case "configure":
		f, e := os.Open(*file)
		if e != nil {
			return e
		}
		defer f.Close()
		cfg, e := config.ParseModels(f)
		if e != nil {
			return e
		}
		v, e := manage("model", cfg)
		if e != nil {
			return e
		}
		return output(v)
	case "import":
		f, e := os.Open(*file)
		if e != nil {
			return e
		}
		defer f.Close()
		return c.Import(ctx, sdk.JSONL{Reader: f})
	case "status":
		r, e := c.Ingestion.GetIngestionStatus(ctx, connect.NewRequest(&trace2memv1.GetIngestionStatusRequest{}))
		if e != nil {
			return e
		}
		return output(r.Msg)
	case "compile":
		r, e := c.Ingestion.RequestCompilation(ctx, connect.NewRequest(&trace2memv1.RequestCompilationRequest{}))
		if e != nil {
			return e
		}
		return output(r.Msg)
	case "search":
		r, e := c.Memory.Search(ctx, connect.NewRequest(&trace2memv1.SearchRequest{Query: *query, Revision: *revision}))
		if e != nil {
			return e
		}
		return output(r.Msg)
	case "context":
		r, e := c.Memory.GetContext(ctx, connect.NewRequest(&trace2memv1.GetContextRequest{Query: *query}))
		if e != nil {
			return e
		}
		if *target != "" {
			ca, e := filesystem.New(ctx, c, r.Msg.Revision, *cache, *max)
			if e != nil {
				return e
			}
			paths := []string{"knowledge/index.md"}
			for _, f := range r.Msg.Files {
				paths = append(paths, f.Path)
			}
			if e = ca.SyncSelected(ctx, *target, paths); e != nil {
				return e
			}
		}
		return output(r.Msg)
	case "forget":
		v, e := manage("forget", map[string]string{"event_id": *event})
		if e != nil {
			return e
		}
		return output(v)
	case "sync", "mount", "export":
		if *target == "" {
			return errors.New("--target required")
		}
		ca, e := filesystem.New(ctx, c, *revision, *cache, *max)
		if e != nil {
			return e
		}
		if command == "mount" {
			if e = os.MkdirAll(*target, 0700); e != nil {
				return e
			}
			for _, f := range ca.Manifest().Files {
				if strings.HasPrefix(f.Path, "knowledge/") {
					if _, e = ca.Read(ctx, f.Path); e != nil {
						return e
					}
				}
			}
			mount, e := filesystem.Mount(ca, *target)
			if e != nil {
				return e
			}
			fmt.Fprintln(os.Stderr, "mounted revision", ca.Manifest().Revision)
			done := make(chan struct{})
			go func() { mount.Wait(); close(done) }()
			select {
			case <-ctx.Done():
				return mount.Unmount()
			case <-done:
				return nil
			}
		}
		if e = ca.Sync(ctx, *target); e != nil {
			return e
		}
		if *git {
			for _, args := range [][]string{{"init"}, {"add", "."}, {"-c", "user.name=Trace2Mem export", "-c", "user.email=export@localhost", "commit", "-m", "Export memory revision " + ca.Manifest().Revision}} {
				cmd := exec.CommandContext(ctx, "git", args...)
				cmd.Dir = *target
				cmd.Stdout = os.Stdout
				cmd.Stderr = os.Stderr
				if e = cmd.Run(); e != nil {
					return e
				}
			}
		}
		return output(map[string]string{"revision": ca.Manifest().Revision, "directory": *target})
	default:
		return errors.New("unknown command: " + command)
	}
}
func env(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}
func exists(p string) bool { _, e := os.Stat(p); return e == nil }
func setup() error {
	if e := os.MkdirAll(".local", 0700); e != nil {
		return e
	}
	for _, name := range []string{"master-key", "bootstrap-token"} {
		p := filepath.Join(".local", name)
		if exists(p) {
			continue
		}
		b := make([]byte, 32)
		if _, e := rand.Read(b); e != nil {
			return e
		}
		if e := os.WriteFile(p, []byte(base64.StdEncoding.EncodeToString(b)), 0600); e != nil {
			return e
		}
	}
	fmt.Println("Local secrets prepared in .local/. Run docker compose up --build -d.")
	return nil
}
