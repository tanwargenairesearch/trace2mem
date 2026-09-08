package server

import (
	"connectrpc.com/connect"
	"context"
	"encoding/json"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	trace2memv1 "github.com/mohit-lendmind/trace2mem/gen/trace2mem/v1"
	"net/http"
)

type memoryArgs struct {
	Query    string `json:"query,omitempty"`
	Path     string `json:"path,omitempty"`
	Revision string `json:"revision,omitempty"`
	EventID  string `json:"event_id,omitempty"`
}

func (s *Server) mcp() http.Handler {
	server := mcp.NewServer(&mcp.Implementation{Name: "trace2mem", Version: "0.1.0"}, nil)
	for _, name := range []string{"memory_index", "memory_search", "memory_read", "memory_evidence", "memory_context", "memory_status"} {
		mcp.AddTool(server, &mcp.Tool{Name: name, Description: "Read authorized memory: " + name}, func(ctx context.Context, req *mcp.CallToolRequest, a memoryArgs) (*mcp.CallToolResult, any, error) {
			var legacy map[string]json.RawMessage
			if err := json.Unmarshal(req.Params.Arguments, &legacy); err != nil {
				return nil, nil, err
			}
			for _, key := range []string{"space_id", "spaceId", "space"} {
				if _, ok := legacy[key]; ok {
					return nil, nil, errLegacySpace
				}
			}
			var result any
			var err error
			switch name {
			case "memory_index":
				r, e := s.ReadFile(ctx, connect.NewRequest(&trace2memv1.ReadFileRequest{Revision: a.Revision, Path: "knowledge/index.md"}))
				if e == nil {
					result = r.Msg
				}
				err = e
			case "memory_search":
				r, e := s.Search(ctx, connect.NewRequest(&trace2memv1.SearchRequest{Query: a.Query, Revision: a.Revision}))
				if e == nil {
					result = r.Msg
				}
				err = e
			case "memory_read":
				r, e := s.ReadFile(ctx, connect.NewRequest(&trace2memv1.ReadFileRequest{Path: a.Path, Revision: a.Revision}))
				if e == nil {
					result = r.Msg
				}
				err = e
			case "memory_evidence":
				r, e := s.GetEvidence(ctx, connect.NewRequest(&trace2memv1.GetEvidenceRequest{EventId: a.EventID}))
				if e == nil {
					result = r.Msg
				}
				err = e
			case "memory_context":
				r, e := s.GetContext(ctx, connect.NewRequest(&trace2memv1.GetContextRequest{Query: a.Query}))
				if e == nil {
					result = r.Msg
				}
				err = e
			case "memory_status":
				r, e := s.GetIngestionStatus(ctx, connect.NewRequest(&trace2memv1.GetIngestionStatusRequest{}))
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
