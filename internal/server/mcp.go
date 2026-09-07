package server

import (
	"connectrpc.com/connect"
	"context"
	"encoding/json"
	brainv1 "github.com/brainmemory/brain/gen/brain/v1"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"net/http"
)

type memoryArgs struct {
	SpaceID  string `json:"space_id" jsonschema:"Memory space identifier"`
	Query    string `json:"query,omitempty"`
	Path     string `json:"path,omitempty"`
	Revision string `json:"revision,omitempty"`
	EventID  string `json:"event_id,omitempty"`
}

func (s *Server) mcp() http.Handler {
	server := mcp.NewServer(&mcp.Implementation{Name: "brain", Version: "0.1.0"}, nil)
	for _, name := range []string{"memory_index", "memory_search", "memory_read", "memory_evidence", "memory_context", "memory_status"} {
		mcp.AddTool(server, &mcp.Tool{Name: name, Description: "Read authorized memory: " + name}, func(ctx context.Context, req *mcp.CallToolRequest, a memoryArgs) (*mcp.CallToolResult, any, error) {
			var result any
			var err error
			switch name {
			case "memory_index":
				r, e := s.ReadFile(ctx, connect.NewRequest(&brainv1.ReadFileRequest{SpaceId: a.SpaceID, Revision: a.Revision, Path: "knowledge/index.md"}))
				if e == nil {
					result = r.Msg
				}
				err = e
			case "memory_search":
				r, e := s.Search(ctx, connect.NewRequest(&brainv1.SearchRequest{SpaceId: a.SpaceID, Query: a.Query, Revision: a.Revision}))
				if e == nil {
					result = r.Msg
				}
				err = e
			case "memory_read":
				r, e := s.ReadFile(ctx, connect.NewRequest(&brainv1.ReadFileRequest{SpaceId: a.SpaceID, Path: a.Path, Revision: a.Revision}))
				if e == nil {
					result = r.Msg
				}
				err = e
			case "memory_evidence":
				r, e := s.GetEvidence(ctx, connect.NewRequest(&brainv1.GetEvidenceRequest{SpaceId: a.SpaceID, EventId: a.EventID}))
				if e == nil {
					result = r.Msg
				}
				err = e
			case "memory_context":
				r, e := s.GetContext(ctx, connect.NewRequest(&brainv1.GetContextRequest{SpaceId: a.SpaceID, Query: a.Query}))
				if e == nil {
					result = r.Msg
				}
				err = e
			case "memory_status":
				r, e := s.GetIngestionStatus(ctx, connect.NewRequest(&brainv1.GetIngestionStatusRequest{SpaceId: a.SpaceID}))
				if e == nil {
					result = r.Msg
				}
				err = e
			}
			if err != nil {
				return nil, nil, err
			}
			b, e := json.Marshal(result)
			if e != nil {
				return nil, nil, e
			}
			return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: string(b)}}}, nil, nil
		})
	}
	return mcp.NewStreamableHTTPHandler(func(r *http.Request) *mcp.Server { return server }, &mcp.StreamableHTTPOptions{Stateless: true})
}
