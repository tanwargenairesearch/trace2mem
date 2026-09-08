package dream

import (
	"encoding/json"
	"fmt"
	"github.com/mohit-lendmind/trace2mem/internal/domain"
	"strings"
)

// The hidden record preserves machine-readable provenance beside the readable note.
func renderNote(o domain.Observation) string {
	o.ID = o.StableID()
	meta, _ := json.Marshal(o)
	var b strings.Builder
	fmt.Fprintf(&b, "<!-- trace2mem-observation %s -->\n# %s\n\n%s\n\nStatus: **%s** · Origin: **%s**\n\nEvidence:", meta, o.Subject, o.Text, o.Status, o.Origin)
	for _, id := range o.Citations {
		fmt.Fprintf(&b, " [cite:%s]", id)
	}
	if len(o.Supersedes) > 0 {
		fmt.Fprintf(&b, "\n\nSupersedes: %s", strings.Join(o.Supersedes, ", "))
	}
	b.WriteString("\n")
	return b.String()
}
func parseNote(content string) (domain.Observation, error) {
	var o domain.Observation
	if strings.HasPrefix(content, "<!-- trace2mem-observation ") {
		line, _, _ := strings.Cut(content, "\n")
		content = strings.TrimSuffix(strings.TrimPrefix(line, "<!-- trace2mem-observation "), " -->")
	}
	err := json.Unmarshal([]byte(content), &o)
	return o, err
}
