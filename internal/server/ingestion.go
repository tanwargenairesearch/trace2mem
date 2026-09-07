package server

import (
	"connectrpc.com/connect"
	"context"
	"encoding/json"
	"errors"
	trace2memv1 "github.com/trace2mem/trace2mem/gen/trace2mem/v1"
	"github.com/trace2mem/trace2mem/internal/domain"
	"github.com/trace2mem/trace2mem/internal/store"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/timestamppb"
	"time"
	"unicode/utf8"
)

func ValidateEvent(v *trace2memv1.Event) error {
	if v == nil || !domain.ValidID(v.EventId) || !domain.ValidID(v.SessionId) {
		return errors.New("valid event and session IDs required")
	}
	if v.Source == nil || !domain.ValidID(v.Source.Id) {
		return errors.New("source ID required")
	}
	if v.OccurredAt == nil || v.OccurredAt.CheckValid() != nil {
		return errors.New("valid occurred_at required")
	}
	switch v.GetActor().GetRole() {
	case "user", "assistant", "tool", "system":
	default:
		return errors.New("actor role must be user, assistant, tool or system")
	}
	switch p := v.Payload.(type) {
	case *trace2memv1.Event_Message:
		if p.Message == nil || p.Message.Text == "" {
			return errors.New("message text required")
		}
	case *trace2memv1.Event_ToolCall:
		if p.ToolCall == nil || !domain.ValidID(p.ToolCall.CallId) || p.ToolCall.Name == "" || !json.Valid([]byte(p.ToolCall.ArgumentsJson)) {
			return errors.New("valid tool call required")
		}
	case *trace2memv1.Event_ToolResult:
		if p.ToolResult == nil || !domain.ValidID(p.ToolResult.CallId) {
			return errors.New("valid tool result required")
		}
	case *trace2memv1.Event_ArtifactReference:
		if p.ArtifactReference == nil || !domain.ValidID(p.ArtifactReference.ArtifactId) {
			return errors.New("artifact ID required")
		}
	case *trace2memv1.Event_SessionLifecycle:
		if p.SessionLifecycle == nil || (p.SessionLifecycle.State != "started" && p.SessionLifecycle.State != "closed") {
			return errors.New("invalid lifecycle state")
		}
	default:
		return errors.New("supported payload required")
	}
	return nil
}
func (s *Server) AppendEvents(ctx context.Context, r *connect.Request[trace2memv1.AppendEventsRequest]) (*connect.Response[trace2memv1.AppendEventsResponse], error) {
	p := principal(ctx)
	if e := s.Store.Authorize(ctx, p, r.Msg.SpaceId, true); e != nil {
		return nil, rpcerr(e)
	}
	if len(r.Msg.Events) == 0 || len(r.Msg.Events) > 256 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("batch must contain 1–256 events"))
	}
	events := make([]store.InputEvent, 0, len(r.Msg.Events))
	for _, v := range r.Msg.Events {
		if e := ValidateEvent(v); e != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, e)
		}
		b, e := protojson.Marshal(v)
		if e != nil {
			return nil, rpcerr(e)
		}
		if len(b) > 64<<10 {
			return nil, connect.NewError(connect.CodeResourceExhausted, errors.New("event exceeds 64 KiB; split large payloads into artifact references"))
		}
		var normalized any
		if e = json.Unmarshal(b, &normalized); e != nil {
			return nil, rpcerr(e)
		}
		canonical, e := json.Marshal(normalized)
		if e != nil {
			return nil, rpcerr(e)
		}
		events = append(events, store.InputEvent{ID: v.EventId, Session: v.SessionId, Hash: domain.Hash(canonical), JSON: canonical, Occurred: v.OccurredAt.AsTime()})
	}
	a, d, w, e := s.Store.Append(ctx, p.Tenant, r.Msg.SpaceId, events)
	if e != nil {
		return nil, rpcerr(e)
	}
	return connect.NewResponse(&trace2memv1.AppendEventsResponse{Accepted: a, Duplicates: d, Watermark: w}), nil
}
func (s *Server) GetIngestionStatus(ctx context.Context, r *connect.Request[trace2memv1.GetIngestionStatusRequest]) (*connect.Response[trace2memv1.GetIngestionStatusResponse], error) {
	p := principal(ctx)
	if e := s.Store.Authorize(ctx, p, r.Msg.SpaceId, false); e != nil {
		return nil, rpcerr(e)
	}
	out := &trace2memv1.GetIngestionStatusResponse{}
	e := s.Store.DB.QueryRow(ctx, `SELECT (SELECT count(*) FROM events WHERE tenant=$1 AND space=$2 AND NOT deleted),(SELECT count(*) FROM events WHERE tenant=$1 AND space=$2 AND NOT deleted AND ordinal<=s.watermark),s.revision,COALESCE(j.status,''),COALESCE(j.error,'') FROM spaces s LEFT JOIN jobs j ON j.tenant=s.tenant AND j.space=s.id WHERE s.tenant=$1 AND s.id=$2`, p.Tenant, r.Msg.SpaceId).Scan(&out.Accepted, &out.Compiled, &out.Revision, &out.JobStatus, &out.LastError)
	out.Pending = out.Accepted - out.Compiled
	return connect.NewResponse(out), rpcerr(e)
}
func (s *Server) UploadArtifact(ctx context.Context, r *connect.Request[trace2memv1.UploadArtifactRequest]) (*connect.Response[trace2memv1.UploadArtifactResponse], error) {
	p := principal(ctx)
	if e := s.Store.Authorize(ctx, p, r.Msg.SpaceId, true); e != nil {
		return nil, rpcerr(e)
	}
	if len(r.Msg.Content) == 0 || len(r.Msg.Content) > 8<<20 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("artifact must contain 1 byte to 8 MiB"))
	}
	if !utf8.Valid(r.Msg.Content) {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("artifact must be valid UTF-8"))
	}
	switch r.Msg.MediaType {
	case "text/plain", "text/markdown", "application/json":
	default:
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("supported artifacts: text/plain, text/markdown, application/json"))
	}
	id := store.ID()
	hash := domain.Hash(r.Msg.Content)
	key := p.Tenant + "/" + r.Msg.SpaceId + "/" + id
	if e := s.Blob.Put(ctx, key, r.Msg.Content); e != nil {
		return nil, rpcerr(e)
	}
	_, e := s.Store.DB.Exec(ctx, "INSERT INTO artifacts VALUES($1,$2,$3,$4,$5,$6,$7)", p.Tenant, r.Msg.SpaceId, id, key, hash, len(r.Msg.Content), r.Msg.MediaType)
	if e != nil {
		cleanup := s.Blob.Delete(ctx, key)
		return nil, rpcerr(errors.Join(e, cleanup))
	}
	return connect.NewResponse(&trace2memv1.UploadArtifactResponse{ArtifactId: id, Sha256: hash}), nil
}
func (s *Server) RequestCompilation(ctx context.Context, r *connect.Request[trace2memv1.RequestCompilationRequest]) (*connect.Response[trace2memv1.RequestCompilationResponse], error) {
	p := principal(ctx)
	if e := s.Store.Authorize(ctx, p, r.Msg.SpaceId, true); e != nil {
		return nil, rpcerr(e)
	}
	e := s.Store.Schedule(ctx, p.Tenant, r.Msg.SpaceId)
	return connect.NewResponse(&trace2memv1.RequestCompilationResponse{Scheduled: e == nil}), rpcerr(e)
}
func (s *Server) CloseSession(ctx context.Context, r *connect.Request[trace2memv1.CloseSessionRequest]) (*connect.Response[trace2memv1.CloseSessionResponse], error) {
	if !domain.ValidID(r.Msg.SessionId) {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid session ID"))
	}
	ev := &trace2memv1.Event{EventId: "close-" + store.ID(), SessionId: r.Msg.SessionId, OccurredAt: timestamppb.New(time.Now()), Actor: &trace2memv1.Actor{Role: "system"}, Source: &trace2memv1.Source{Id: "trace2mem-api", Format: "trace2mem.v1"}, Payload: &trace2memv1.Event_SessionLifecycle{SessionLifecycle: &trace2memv1.SessionLifecycle{State: "closed"}}}
	_, e := s.AppendEvents(ctx, connect.NewRequest(&trace2memv1.AppendEventsRequest{SpaceId: r.Msg.SpaceId, Events: []*trace2memv1.Event{ev}}))
	return connect.NewResponse(&trace2memv1.CloseSessionResponse{Scheduled: e == nil}), e
}
