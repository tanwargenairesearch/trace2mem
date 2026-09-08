package dream

import (
	"github.com/tanwargenairesearch/trace2mem/internal/domain"
	"strings"
	"testing"
)

func TestReadableNotePreservesProvenance(t *testing.T) {
	o := domain.Observation{Subject: "Project", Text: "Use Go", Origin: "user", Status: "current", Citations: []string{"e1"}, Supersedes: []string{"old"}}
	text := renderNote(o)
	if !strings.Contains(text, "# Project\n\nUse Go") || !strings.Contains(text, "[cite:e1]") {
		t.Fatal("note is not readable Markdown")
	}
	got, err := parseNote(text)
	if err != nil || got.ID != o.StableID() || got.Supersedes[0] != "old" {
		t.Fatal("provenance lost", got, err)
	}
}
func TestDraftRejectsMissingAndForeignEvidence(t *testing.T) {
	records := []domain.Record{{ID: "e1", Session: "s1", Role: "user", Text: "Project uses Go"}}
	obs := []domain.Observation{{Subject: "Project", Text: "uses Go", Origin: "user", Status: "current", Citations: []string{"e1"}}}
	draft := wikiDraft{Sessions: []draftPage{{Name: "s1", Text: "The user selected Go [cite:e1]."}}, Subjects: []draftPage{{Name: "Project", Text: "The project uses Go [cite:e1]."}}}
	if _, err := validateDraft(draft, records, obs); err != nil {
		t.Fatal(err)
	}
	draft.Subjects[0].Text = "An unrelated assertion [cite:foreign]."
	if _, err := validateDraft(draft, records, obs); err == nil {
		t.Fatal("foreign evidence accepted")
	}
	draft.Subjects = nil
	if _, err := validateDraft(draft, records, obs); err == nil {
		t.Fatal("missing subject accepted")
	}
}
