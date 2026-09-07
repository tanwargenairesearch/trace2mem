package dream

import (
	"github.com/brainmemory/brain/internal/domain"
	"testing"
)

func TestRejectUnsupportedAndMisattributedClaims(t *testing.T) {
	records := []domain.Record{{ID: "e1", Role: "assistant", Text: "Launch October"}}
	o := domain.Observation{Subject: "launch", Text: "October", Origin: "user", Status: "current", Citations: []string{"e1"}}
	if Validate([]domain.Observation{o}, records) == nil {
		t.Fatal("accepted assistant assertion as user fact")
	}
	o.Origin = "assistant"
	o.Citations = []string{"missing"}
	if Validate([]domain.Observation{o}, records) == nil {
		t.Fatal("accepted dangling citation")
	}
	o.Citations = []string{"e1"}
	if e := Validate([]domain.Observation{o}, records); e != nil {
		t.Fatal(e)
	}
}
func TestPathsAndEvidence(t *testing.T) {
	r := []domain.Record{{ID: "e1", Session: "s1", Role: "user", Text: "October"}}
	o := []domain.Observation{{Subject: "../../launch", Text: "October", Origin: "user", Status: "current", Citations: []string{"e1"}}}
	pages := Build(o, r)
	seen := map[string]bool{}
	for _, p := range pages {
		if !domain.ValidPath(p.Path) {
			t.Fatalf("unsafe path %s", p.Path)
		}
		seen[p.Path] = true
	}
	for _, p := range []string{"knowledge/index.md", "sessions/evidence/e1.json", "sessions/s1/summary.md"} {
		if !seen[p] {
			t.Errorf("missing %s", p)
		}
	}
}

func TestObservationIdentitySeparatesFactsFromSameSource(t *testing.T) {
	a := domain.Observation{Subject: "project", Text: "uses Go", Origin: "user", Citations: []string{"event1"}}
	b := a
	b.Text = "deadline Friday"
	if a.StableID() == b.StableID() {
		t.Fatal("distinct facts share identity")
	}
	c := a
	c.Status = "superseded"
	if a.StableID() != c.StableID() {
		t.Fatal("temporal status changed identity")
	}
}
