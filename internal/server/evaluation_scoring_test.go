package server

import "testing"

func TestFactScoringRequiresAttachedSupportingEvidence(t *testing.T) {
	c := evalCase{Query: "deadline", Split: "heldout", Facts: []expectedFact{{ID: "deadline", AnswerPattern: `(?i)deadline is October`, EvidencePattern: `Launch: October`, Citations: []string{"e2"}, ForbiddenPatterns: []string{`(?i)deadline is September`}}}}
	if err := validateCases([]evalCase{c}); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		answer, source string
		pass           bool
	}{
		{"The deadline is October [cite:e2]", "Launch: October", true},
		{"The deadline is October [cite:e2]", "Launch: September", false},
		{"The deadline is October\nUnrelated [cite:e2]", "Launch: October", false},
		{"The deadline is October [cite:e1]", "Launch: October", false},
		{"The deadline is October [cite:e2], but the deadline is September", "Launch: October", false},
		{"September remains the deadline; October was rejected [cite:e2]", "Launch: October", false},
	} {
		_, _, pass := scoreFacts(c, tc.answer, map[string]string{"e2": tc.source})
		if pass != tc.pass {
			t.Errorf("%q: passed=%v", tc.answer, pass)
		}
	}
	c.Facts[0].AnswerPattern = ".*"
	if validateCases([]evalCase{c}) == nil {
		t.Fatal("empty-match rubric accepted")
	}
}
