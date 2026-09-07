package server

import (
	"connectrpc.com/connect"
	"context"
	"encoding/json"
	"errors"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/proto"
)

var errLegacySpace = errors.New("spaces were removed; upgrade your client to per-user memory")

type memoryJSONCodec struct{}

func (memoryJSONCodec) Name() string                  { return "json" }
func (memoryJSONCodec) Marshal(v any) ([]byte, error) { return protojson.Marshal(v.(proto.Message)) }
func (memoryJSONCodec) Unmarshal(data []byte, v any) error {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return err
	}
	for _, key := range []string{"spaceId", "space_id", "space"} {
		if _, ok := fields[key]; ok {
			return errLegacySpace
		}
	}
	return protojson.Unmarshal(data, v.(proto.Message))
}
func rejectLegacySpace(next connect.UnaryFunc) connect.UnaryFunc {
	return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
		if msg, ok := req.Any().(proto.Message); ok {
			data := msg.ProtoReflect().GetUnknown()
			for len(data) > 0 {
				num, typ, n := protowire.ConsumeTag(data)
				if n < 0 {
					return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid protobuf field"))
				}
				if num == 1 {
					return nil, connect.NewError(connect.CodeInvalidArgument, errLegacySpace)
				}
				data = data[n:]
				n = protowire.ConsumeFieldValue(num, typ, data)
				if n < 0 {
					return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid protobuf field"))
				}
				data = data[n:]
			}
		}
		return next(ctx, req)
	}
}
