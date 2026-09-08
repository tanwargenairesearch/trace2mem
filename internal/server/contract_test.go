package server

import (
	"connectrpc.com/connect"
	"context"
	v1 "github.com/mohit-lendmind/trace2mem/gen/trace2mem/v1"
	"google.golang.org/protobuf/encoding/protowire"
	"testing"
)

func TestLegacyMemorySelectionRejected(t *testing.T) {
	for _, body := range []string{`{"spaceId":"old"}`, `{"space_id":""}`, `{"space":null}`} {
		if err := (memoryJSONCodec{}).Unmarshal([]byte(body), &v1.GetManifestRequest{}); err == nil {
			t.Fatalf("accepted legacy JSON: %s", body)
		}
	}
	req := &v1.GetManifestRequest{}
	req.ProtoReflect().SetUnknown(protowire.AppendString(protowire.AppendTag(nil, 1, protowire.BytesType), "old"))
	called := false
	handler := rejectLegacySpace(func(context.Context, connect.AnyRequest) (connect.AnyResponse, error) { called = true; return nil, nil })
	if _, err := handler(context.Background(), connect.NewRequest(req)); connect.CodeOf(err) != connect.CodeInvalidArgument || called {
		t.Fatal("legacy protobuf reached handler", err)
	}
	if err := (memoryJSONCodec{}).Unmarshal([]byte(`{"revision":"r1"}`), &v1.GetManifestRequest{}); err != nil {
		t.Fatal(err)
	}
}
