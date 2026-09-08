package server

import (
	"errors"
	"regexp"
	"strings"

	"github.com/mohit-lendmind/trace2mem/internal/domain"
)

type expectedFact struct {
	StatusPattern      string   `json:"status_pattern,omitempty"`
	AttributionPattern string   `json:"attribution_pattern,omitempty"`
	ForbidNegation     bool     `json:"forbid_negation,omitempty"`
	ID                 string   `json:"id"`
	AnswerPattern      string   `json:"answer_pattern"`
	EvidencePattern    string   `json:"evidence_pattern"`
	Citations          []string `json:"citations"`
	ForbiddenPatterns  []string `json:"forbidden_patterns,omitempty"`
}

type factScore struct {
	ContextMatches   bool   `json:"status_and_attribution_match"`
	NegationDetected bool   `json:"negation_detected"`
	ID               string `json:"id"`
	AnswerMatches    bool   `json:"answer_matches"`
	EvidenceMatches  bool   `json:"evidence_matches"`
	CitationAttached bool   `json:"citation_attached"`
	ForbiddenMatch   bool   `json:"forbidden_match"`
	Passed           bool   `json:"passed"`
}

func validateCases(cases []evalCase) error {
	if len(cases) < 1 || len(cases) > 20 {
		return errors.New("provide 1–20 cases")
	}
	for _, c := range cases {
		if len(c.Query) == 0 || len(c.Query) > 4096 || (c.Split != "development" && c.Split != "heldout") || len(c.Facts) == 0 || len(c.Facts) > 20 {
			return errors.New("each case needs query, development/heldout split and 1–20 expected facts")
		}
		seen := map[string]bool{}
		for _, f := range c.Facts {
			if !domain.ValidID(f.ID) || seen[f.ID] || len(f.Citations) == 0 || len(f.Citations) > 20 || len(f.ForbiddenPatterns) > 20 {
				return errors.New("facts need unique IDs and 1–20 supporting citations")
			}
			seen[f.ID] = true
			ids := map[string]bool{}
			for _, id := range f.Citations {
				if !domain.ValidID(id) || ids[id] {
					return errors.New("invalid or duplicate fact citation")
				}
				ids[id] = true
			}
			patterns := append([]string{f.AnswerPattern, f.EvidencePattern}, f.ForbiddenPatterns...)
			if f.StatusPattern != "" {
				patterns = append(patterns, f.StatusPattern)
			}
			if f.AttributionPattern != "" {
				patterns = append(patterns, f.AttributionPattern)
			}
			for _, pattern := range patterns {
				if pattern == "" || len(pattern) > 1024 {
					return errors.New("fact patterns must contain 1–1024 bytes")
				}
				re, err := regexp.Compile(pattern)
				if err != nil || re.MatchString("") {
					return errors.New("fact patterns must be valid nonempty RE2 matches")
				}
			}
		}
	}
	return nil
}

// scoreFacts is an explicit lexical rubric, not an independent semantic judge.
// A supporting citation must appear on the same line as the expected assertion.
func scoreFacts(c evalCase, answer string, evidence map[string]string) ([]factScore, float64, bool) {
	var scores []factScore
	all := true
	expected, recalled := map[string]bool{}, map[string]bool{}
	for _, f := range c.Facts {
		x := factScore{ID: f.ID}
		ar, er := regexp.MustCompile(f.AnswerPattern), regexp.MustCompile(f.EvidencePattern)
		x.AnswerMatches = ar.MatchString(answer)
		for _, pattern := range f.ForbiddenPatterns {
			x.ForbiddenMatch = x.ForbiddenMatch || regexp.MustCompile(pattern).MatchString(answer)
		}
		for _, id := range f.Citations {
			expected[id] = true
			marker := "[cite:" + id + "]"
			if strings.Contains(answer, marker) {
				recalled[id] = true
			}
			if !er.MatchString(evidence[id]) {
				continue
			}
			x.EvidenceMatches = true
			for _, line := range strings.Split(answer, "\n") {
				if ar.MatchString(line) && strings.Contains(line, marker) {
					x.CitationAttached = true
					contextOK := true
					for _, pattern := range []string{f.StatusPattern, f.AttributionPattern} {
						if pattern != "" && !regexp.MustCompile(pattern).MatchString(line) {
							contextOK = false
						}
					}
					negated := f.ForbidNegation && negationPattern.MatchString(line)
					x.NegationDetected = x.NegationDetected || negated
					x.ContextMatches = x.ContextMatches || (contextOK && !negated)
				}
			}
		}
		x.Passed = x.AnswerMatches && x.EvidenceMatches && x.CitationAttached && x.ContextMatches && !x.NegationDetected && !x.ForbiddenMatch
		all = all && x.Passed
		scores = append(scores, x)
	}
	return scores, float64(len(recalled)) / float64(len(expected)), all
}

// Opt-in conservative English lexical filter; structured task rubrics remain preferable.
var negationPattern = regexp.MustCompile(`(?i)\b(not|never|no longer|false|incorrect)\b`)
