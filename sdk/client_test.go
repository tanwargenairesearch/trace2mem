package sdk

import (
	"connectrpc.com/connect"
	"context"
	"encoding/json"
	v1 "github.com/mohit-lendmind/trace2mem/gen/trace2mem/v1"
	"net/http"
	"net/http/httptest"
	"testing"
)

type reusedEvents struct{}

func (reusedEvents) Read(ctx context.Context, emit func(*v1.Event) error) error {
	event := &v1.Event{Payload: &v1.Event_Message{Message: &v1.Message{}}}
	for _, id := range []string{"first", "second"} {
		event.EventId = id
		event.GetMessage().Text = id
		if err := emit(event); err != nil {
			return err
		}
	}
	return nil
}
func TestImportSnapshotsReusedEnvelope(t *testing.T) {
	var got struct {
		Events []struct {
			ID      string `json:"eventId"`
			Message struct {
				Text string `json:"text"`
			} `json:"message"`
		} `json:"events"`
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Error(err)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"accepted":"2"}`))
	}))
	defer srv.Close()
	if err := New(srv.URL, "fixture", connect.WithProtoJSON()).Import(context.Background(), reusedEvents{}); err != nil {
		t.Fatal(err)
	}
	if len(got.Events) != 2 {
		t.Fatal(got)
	}
	for i, id := range []string{"first", "second"} {
		if got.Events[i].ID != id || got.Events[i].Message.Text != id {
			t.Fatalf("event %d changed after capture: %+v", i, got.Events[i])
		}
	}
}
