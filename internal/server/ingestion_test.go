package server

import (
	brainv1 "github.com/brainmemory/brain/gen/brain/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
	"testing"
)

func TestEventValidation(t *testing.T) {
	e := &brainv1.Event{EventId: "e1", SessionId: "s1", Source: &brainv1.Source{Id: "adapter"}, Actor: &brainv1.Actor{Role: "user"}, OccurredAt: timestamppb.Now(), Payload: &brainv1.Event_Message{Message: &brainv1.Message{Text: "hello"}}}
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
