package server

import (
	trace2memv1 "github.com/tanwargenairesearch/trace2mem/gen/trace2mem/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
	"testing"
)

func TestEventValidation(t *testing.T) {
	e := &trace2memv1.Event{EventId: "e1", SessionId: "s1", Source: &trace2memv1.Source{Id: "adapter"}, Actor: &trace2memv1.Actor{Role: "user"}, OccurredAt: timestamppb.Now(), Payload: &trace2memv1.Event_Message{Message: &trace2memv1.Message{Text: "hello"}}}
	if err := ValidateEvent(e); err != nil {
		t.Fatal(err)
	}
	e.EventId = "../other"
	if ValidateEvent(e) == nil {
		t.Fatal("unsafe source path accepted")
	}
	e.EventId = "e1"
	e.Payload = nil
	if ValidateEvent(e) == nil {
		t.Fatal("missing payload accepted")
	}
}
func TestSynthesisCannotInventCitations(t *testing.T) {
	if validateSynthesis("Claim [cite:missing]", map[string]bool{"e1": true}) == nil {
		t.Fatal("uninspected citation accepted")
	}
	if validateSynthesis("Uncited assertion", map[string]bool{"e1": true}) == nil {
		t.Fatal("uncited synthesis accepted")
	}
	if e := validateSynthesis("Claim [cite:e1]", map[string]bool{"e1": true}); e != nil {
		t.Fatal(e)
	}
}
