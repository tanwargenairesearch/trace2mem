package live

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"connectrpc.com/connect"
	v1 "github.com/mohit-lendmind/trace2mem/gen/trace2mem/v1"
	"github.com/mohit-lendmind/trace2mem/internal/blob"
	"github.com/mohit-lendmind/trace2mem/internal/config"
	"github.com/mohit-lendmind/trace2mem/internal/domain"
	"github.com/mohit-lendmind/trace2mem/internal/dream"
	"github.com/mohit-lendmind/trace2mem/internal/server"
	"github.com/mohit-lendmind/trace2mem/internal/store"
	"github.com/mohit-lendmind/trace2mem/sdk"
	"google.golang.org/protobuf/encoding/protojson"
)

// TestPersonaCompilation compiles authored histories without reading evaluation questions.
func TestPersonaCompilation(t *testing.T) {
	dsn, modelPath, input, output := os.Getenv("TRACE2MEM_LIVE_DATABASE"), os.Getenv("TRACE2MEM_LIVE_CONFIG"), os.Getenv("TRACE2MEM_PERSONA_INPUT"), os.Getenv("TRACE2MEM_LIVE_REPORT_DIR")
	if dsn == "" || modelPath == "" || input == "" || output == "" {
		t.Skip("opt-in: dedicated database, model config, persona input and report directory required")
	}
	for _, id := range []string{"nadia", "marco", "leena", "owen"} {
		t.Run(id, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 6*time.Minute)
			defer cancel()
			folder := filepath.Join(output, id)
			if _, err := os.Stat(folder); !os.IsNotExist(err) {
				t.Fatal("choose a fresh output directory")
			}
			if err := os.MkdirAll(folder, 0700); err != nil {
				t.Fatal(err)
			}
			f, err := os.Open(modelPath)
			if err != nil {
				t.Fatal(err)
			}
			cfg, err := config.ParseModels(f)
			f.Close()
			if err != nil {
				t.Fatal(err)
			}
			cfg.Prompt = "Maintain concise user memory across all supplied sessions: preferences, confirmed plans, changes, ownership, unapproved suggestions and explicit unknowns. Propose at most 14 concise observations. Preserve original actor attribution, temporal status and exact source citations. Raw conversations remain accessible."
			s, err := store.Open(ctx, dsn)
			if err != nil {
				t.Fatal(err)
			}
			defer s.Close()
			if err = s.Migrate(ctx); err != nil {
				t.Fatal(err)
			}
			p := domain.Principal{Tenant: "persona-" + store.ID(), Subject: id, Scopes: map[string]bool{"read": true, "ingest": true, "manage": true}}
			if err = s.EnsureMemory(ctx, p); err != nil {
				t.Fatal(err)
			}
			if err = s.SetSchedule(ctx, p.Tenant, p.MemoryID(), store.CompilationSchedule{Mode: "manual"}); err != nil {
				t.Fatal(err)
			}
			vault, err := config.NewVault(base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{42}, 32)))
			if err != nil {
				t.Fatal(err)
			}
			keys, err := json.Marshal(map[string]string{"generation": cfg.Key, "embedding": cfg.EmbeddingKey})
			if err != nil {
				t.Fatal(err)
			}
			sealed, err := vault.Seal(ctx, keys, []byte(p.Tenant+"/"+p.MemoryID()))
			if err != nil {
				t.Fatal(err)
			}
			if err = s.SetConfig(ctx, p.Tenant, p.MemoryID(), cfg, sealed); err != nil {
				t.Fatal(err)
			}
			result := map[string]any{"history_id": id, "published": false, "model": cfg.Model, "embedding_model": cfg.EmbeddingModel, "maintenance_prompt": cfg.Prompt, "max_tokens": cfg.MaxTokens, "daily_tokens": cfg.DailyTokens, "max_steps": cfg.MaxSteps}
			started := time.Now()
			defer func() {
				auditCtx, auditCancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer auditCancel()
				result["elapsed_ms"] = time.Since(started).Milliseconds()
				var usage, proposals json.RawMessage
				err = s.DB.QueryRow(auditCtx, `SELECT COALESCE(jsonb_agg(x),'[]'::jsonb) FROM (SELECT operation,sum(input_tokens) AS input_tokens,sum(output_tokens) AS output_tokens,bool_or(estimated) AS estimated FROM usage WHERE tenant=$1 AND space=$2 GROUP BY operation) x`, p.Tenant, p.MemoryID()).Scan(&usage)
				if err != nil {
					t.Error(err)
				} else {
					result["usage"] = usage
				}
				err = s.DB.QueryRow(auditCtx, `SELECT COALESCE(jsonb_agg(jsonb_build_object('status',status,'content',content,'verification',verification)),'[]'::jsonb) FROM proposals WHERE tenant=$1 AND space=$2`, p.Tenant, p.MemoryID()).Scan(&proposals)
				if err != nil {
					t.Error(err)
				} else if err = os.WriteFile(filepath.Join(folder, "proposals.json"), proposals, 0600); err != nil {
					t.Error(err)
				}
				b, err := json.MarshalIndent(result, "", "  ")
				if err != nil {
					t.Error(err)
				} else if err = os.WriteFile(filepath.Join(folder, "compilation.json"), b, 0600); err != nil {
					t.Error(err)
				}
			}()
			blobs := blob.Local{Root: t.TempDir()}
			operator := config.Config{AllowedEndpoints: cfg.Endpoint + "," + cfg.EmbeddingEndpoint, Scripted: cfg.Provider == "scripted"}
			engine := &dream.Engine{Store: s, Vault: vault, Blob: blobs, Config: operator}
			api := &server.Server{Store: s, Vault: vault, Blob: blobs, Config: operator, Engine: engine}
			host := httptest.NewServer(api.Handler())
			defer host.Close()
			token, err := s.CreateToken(ctx, p, []string{"read", "ingest", "manage"})
			if err != nil {
				t.Fatal(err)
			}
			client := sdk.New(host.URL, token)
			f, err = os.Open(filepath.Join(input, id, "history.jsonl"))
			if err != nil {
				t.Fatal(err)
			}
			var events []*v1.Event
			scan := bufio.NewScanner(f)
			scan.Buffer(make([]byte, 4096), 128<<10)
			totalBytes := 0
			for scan.Scan() {
				totalBytes += len(scan.Bytes())
				if len(events) >= 18 || totalBytes > 128<<10 {
					f.Close()
					t.Fatal("persona input limit")
				}
				var ev v1.Event
				if err = protojson.Unmarshal(scan.Bytes(), &ev); err != nil {
					f.Close()
					t.Fatal(err)
				}
				events = append(events, &ev)
			}
			scanErr := scan.Err()
			f.Close()
			if scanErr != nil {
				t.Fatal(scanErr)
			}
			if len(events) != 18 {
				t.Fatal("expected authored 18-event history")
			}
			if _, err = client.Ingestion.AppendEvents(ctx, connect.NewRequest(&v1.AppendEventsRequest{Events: events})); err != nil {
				t.Fatal(err)
			}
			if err = s.Schedule(ctx, p.Tenant, p.MemoryID()); err != nil {
				t.Fatal(err)
			}
			lease, err := s.Claim(ctx)
			if err != nil || lease == nil {
				t.Fatal("claim", err)
			}
			if lease.Tenant != p.Tenant || lease.Space != p.MemoryID() {
				t.Fatal("dedicated database required")
			}
			if err = engine.Run(ctx, *lease); err != nil {
				result["error"] = fmt.Sprint(err)
				if failureErr := terminalPersonaFailure(s, *lease); failureErr != nil {
					t.Fatal("terminalize failed compilation", failureErr)
				}
				t.Fatal("compilation", err)
			}
			manifest, err := client.Memory.GetManifest(ctx, connect.NewRequest(&v1.GetManifestRequest{}))
			if err != nil {
				t.Fatal(err)
			}
			for _, file := range manifest.Msg.Files {
				if !domain.ValidPath(file.Path) {
					t.Fatal("unsafe manifest path")
				}
				res, err := client.Memory.ReadFile(ctx, connect.NewRequest(&v1.ReadFileRequest{Revision: manifest.Msg.Revision, Path: file.Path}))
				if err != nil {
					t.Fatal(err)
				}
				path := filepath.Join(folder, "memory", file.Path)
				if err = os.MkdirAll(filepath.Dir(path), 0700); err != nil {
					t.Fatal(err)
				}
				if err = os.WriteFile(path, []byte(res.Msg.Content), 0600); err != nil {
					t.Fatal(err)
				}
			}
			b, err := protojson.Marshal(manifest.Msg)
			if err != nil {
				t.Fatal(err)
			}
			if err = os.WriteFile(filepath.Join(folder, "manifest.json"), b, 0600); err != nil {
				t.Fatal(err)
			}
			result["published"] = true
			result["revision"] = manifest.Msg.Revision
			result["watermark"] = manifest.Msg.Watermark
		})
	}
}

// A benchmark failure must not become another persona's automatically retried job.
func terminalPersonaFailure(s *store.Store, l domain.Lease) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	tag, err := s.DB.Exec(ctx, "UPDATE jobs SET status='failed',lease_until=NULL,error='persona compilation failed; no automatic benchmark retry' WHERE tenant=$1 AND space=$2 AND fence=$3 AND status='running'", l.Tenant, l.Space, l.Fence)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return fmt.Errorf("failed job lost its fence")
	}
	return nil
}

func TestPersonaFailureIsolation(t *testing.T) {
	dsn := os.Getenv("TRACE2MEM_TEST_DATABASE")
	if dsn == "" {
		t.Skip("disposable database required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	s, err := store.Open(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err = s.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		p := domain.Principal{Tenant: "failure-" + store.ID(), Subject: "fixture"}
		if err = s.EnsureMemory(ctx, p); err != nil {
			t.Fatal(err)
		}
		cfg := domain.ModelConfig{Provider: "scripted", Model: "fixture", EmbeddingProvider: "scripted", EmbeddingModel: "fixture"}
		if err = s.SetConfig(ctx, p.Tenant, p.MemoryID(), cfg, nil); err != nil {
			t.Fatal(err)
		}
		if err = s.Schedule(ctx, p.Tenant, p.MemoryID()); err != nil {
			t.Fatal(err)
		}
		lease, err := s.Claim(ctx)
		if err != nil || lease == nil {
			t.Fatal("claim", err)
		}
		if lease.Tenant != p.Tenant {
			t.Fatal("prior failure reclaimed")
		}
		if err = terminalPersonaFailure(s, *lease); err != nil {
			t.Fatal(err)
		}
		if _, err = s.DB.Exec(ctx, "UPDATE jobs SET lease_until=now()-interval '1 minute',available_at=now()-interval '1 minute' WHERE tenant=$1 AND space=$2", p.Tenant, p.MemoryID()); err != nil {
			t.Fatal(err)
		}
	}
}
