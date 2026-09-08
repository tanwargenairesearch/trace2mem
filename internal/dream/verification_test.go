package dream

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/tanwargenairesearch/trace2mem/internal/domain"
	"github.com/tanwargenairesearch/trace2mem/internal/model"
	"github.com/tanwargenairesearch/trace2mem/internal/store"
)

func TestWikiJudgmentRequiresEveryPassageAndTemporalSupport(t *testing.T) {
	units := []reviewUnit{{ID: "knowledge/subjects/harbor.md:1", Text: "The approved v1 decisions were tentative [cite:recap].", Citations: []string{"recap"}}}
	var err error
	sources := []domain.Record{{ID: "approval", Role: "user", Text: "Approve the v1 decisions."}, {ID: "recap", Role: "assistant", Text: "Previously tentative decisions."}}
	judgment := wikiJudgment{Supported: true, Reason: "overall fixture verdict", Reviews: []passageReview{{ID: units[0].ID, Supported: true, AttributionValid: true, TemporalValid: false, Sources: []string{"recap"}, Conflicts: []string{"approval"}, Reason: "Assistant recap contradicts the earlier explicit approval"}}}
	if validateWikiJudgment(judgment, units, sources) == nil {
		t.Fatal("overall verdict bypassed temporal conflict")
	}
	judgment.Reviews[0].TemporalValid = true
	if err = validateWikiJudgment(judgment, units, sources); err != nil {
		t.Fatal(err)
	}
	judgment.Reviews[0].Conflicts = []string{"Duplicate citation token: cosmetic, no factual impact."}
	if validateWikiJudgment(judgment, units, sources) == nil {
		t.Fatal("prose accepted as an evidence ID")
	}
	judgment.Reviews = nil
	if validateWikiJudgment(judgment, units, sources) == nil {
		t.Fatal("missing passage review accepted")
	}
	// The fixture tests the gate; it does not establish that a real model detects the conflict.
}

func TestPriorPaginationAndUnchangedNeighbors(t *testing.T) {
	dsn := os.Getenv("TRACE2MEM_TEST_DATABASE")
	if dsn == "" {
		t.Skip("requires test database")
	}
	ctx := context.Background()
	db, err := store.Open(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err = db.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	p := domain.Principal{Tenant: "investigate-" + store.ID(), Subject: "u"}
	if err = db.EnsureMemory(ctx, p); err != nil {
		t.Fatal(err)
	}
	if _, err = db.DB.Exec(ctx, "INSERT INTO revisions(tenant,space,id,parent,watermark,verification) VALUES($1,$2,'rev','',1,'{}')", p.Tenant, p.MemoryID()); err != nil {
		t.Fatal(err)
	}
	if _, err = db.DB.Exec(ctx, "INSERT INTO events(tenant,space,id,session,hash,payload,occurred_at) VALUES($1,$2,'approval','s1','fixture',$3,now())", p.Tenant, p.MemoryID(), json.RawMessage(`{"eventId":"approval","actor":{"role":"user"},"message":{"text":"Approved."}}`)); err != nil {
		t.Fatal(err)
	}
	insert := func(path, content string) {
		t.Helper()
		if _, err := db.DB.Exec(ctx, "INSERT INTO pages(tenant,space,revision,path,content,hash,citations) VALUES($1,$2,'rev',$3,$4,'fixture',$5)", p.Tenant, p.MemoryID(), path, content, []string{"approval"}); err != nil {
			t.Fatal(err)
		}
	}
	for i := 0; i < 70; i++ {
		content := fmt.Sprintf(`{"subject":"Project","text":"Topic %d","origin":"user","status":"current","citations":["approval"]}`, i)
		insert(fmt.Sprintf("notes/project/%03d.md", i), content)
	}
	e := Engine{Store: db}
	lease := domain.Lease{Tenant: p.Tenant, Space: p.MemoryID(), Parent: "rev"}
	first, _, err := e.prior(ctx, lease, "Topic", "")
	if err != nil {
		t.Fatal(err)
	}
	var a struct {
		Notes  []domain.Observation `json:"notes"`
		More   bool                 `json:"truncated"`
		Cursor string               `json:"next_cursor"`
	}
	if err = json.Unmarshal([]byte(first), &a); err != nil {
		t.Fatal(err)
	}
	if len(a.Notes) != 64 || !a.More || a.Cursor == "" {
		t.Fatal(first)
	}
	second, _, err := e.prior(ctx, lease, "Topic", a.Cursor)
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal([]byte(second), &a); err != nil {
		t.Fatal(err)
	}
	if len(a.Notes) != 6 || a.More {
		t.Fatal(second)
	}
	for i := 0; i < 2; i++ {
		body, _ := json.Marshal(map[string]any{"subject": "Large", "text": strings.Repeat("x", 70000), "origin": "user", "status": "current", "citations": []string{"approval"}})
		insert(fmt.Sprintf("notes/large/%d.md", i), string(body))
	}
	large, _, err := e.prior(ctx, lease, "Large", "")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal([]byte(large), &a); err != nil {
		t.Fatal(err)
	}
	if len(a.Notes) != 1 || !a.More {
		t.Fatal("byte budget did not return a partial page")
	}
	large, _, err = e.prior(ctx, lease, "Large", a.Cursor)
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal([]byte(large), &a); err != nil {
		t.Fatal(err)
	}
	if len(a.Notes) != 1 || a.More {
		t.Fatal("byte continuation lost a note")
	}
	insert("knowledge/subjects/unchanged.md", "Already approved [cite:approval]. Related: [[knowledge/subjects/changed.md]]")
	neighbors, records, err := e.neighbors(ctx, lease, map[string]domain.Page{"knowledge/subjects/changed.md": {Content: "A changed decision [cite:approval]."}})
	if err != nil {
		t.Fatal(err)
	}
	if len(neighbors) != 1 || len(records) != 1 || !strings.Contains(neighbors["knowledge/subjects/unchanged.md"], "Already approved") {
		t.Fatal(neighbors, records)
	}
}

func TestCompilerToolSchemasRequireEveryProperty(t *testing.T) {
	var check func(map[string]any)
	check = func(schema map[string]any) {
		if props, ok := schema["properties"].(map[string]any); ok {
			required, _ := schema["required"].([]string)
			keys := map[string]bool{}
			for _, key := range required {
				keys[key] = true
			}
			for key, value := range props {
				if !keys[key] {
					t.Errorf("strict schema omitted required property %s", key)
				}
				if child, ok := value.(map[string]any); ok {
					check(child)
				}
			}
		}
		if child, ok := schema["items"].(map[string]any); ok {
			check(child)
		}
	}
	for _, tool := range compilerTools() {
		check(tool.Parameters)
	}
}

func TestReviewUnitsIncludeEveryNonemptyLine(t *testing.T) {
	units, err := reviewUnits(map[string]domain.Page{"knowledge/p.md": {Content: "# Title\n\nApproved [cite:e1].\n"}})
	if err != nil || len(units) != 2 || units[0].ID != "knowledge/p.md:1" || units[1].ID != "knowledge/p.md:3" || units[1].Citations[0] != "e1" {
		t.Fatal(units, err)
	}
}

type verificationSchemaProbe struct{ t *testing.T }

func (p verificationSchemaProbe) Generate(_ context.Context, _ []model.Turn, tools []model.Tool) (model.Reply, error) {
	entry := tools[0].Parameters["properties"].(map[string]any)["reviews"].(map[string]any)["items"].(map[string]any)
	for _, name := range []string{"sources", "conflicts"} {
		schema := entry["properties"].(map[string]any)[name].(map[string]any)
		enum := schema["items"].(map[string]any)["enum"].([]string)
		if len(enum) != 2 || enum[0] != "approval" || enum[1] != "recap" {
			p.t.Fatal("evidence IDs not constrained", enum)
		}
	}
	return model.Reply{}, fmt.Errorf("schema probe complete")
}
func (verificationSchemaProbe) Embed(context.Context, []string) ([][]float32, error) {
	return nil, fmt.Errorf("unused")
}

func TestVerificationSourceSchemaExcludesProse(t *testing.T) {
	e := Engine{}
	_, err := e.verifyWiki(context.Background(), domain.Lease{}, verificationSchemaProbe{t}, map[string]domain.Page{"knowledge/subjects/x.md": {Content: "Approved [cite:approval]."}}, []domain.Record{{ID: "recap"}, {ID: "approval"}}, map[string]any{})
	if err == nil || err.Error() != "schema probe complete" {
		t.Fatal(err)
	}
}
