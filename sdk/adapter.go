// Package sdk provides a transport-independent event adapter contract and Connect clients.
package sdk

import (
	"bufio"
	"context"
	"fmt"
	trace2memv1 "github.com/mohit-lendmind/trace2mem/gen/trace2mem/v1"
	"google.golang.org/protobuf/encoding/protojson"
	"io"
)

type Adapter interface {
	Read(context.Context, func(*trace2memv1.Event) error) error
}
type JSONL struct{ Reader io.Reader }

func (a JSONL) Read(ctx context.Context, emit func(*trace2memv1.Event) error) error {
	s := bufio.NewScanner(a.Reader)
	s.Buffer(make([]byte, 4096), 1<<20)
	line := 0
	for s.Scan() {
		line++
		if e := ctx.Err(); e != nil {
			return e
		}
		if len(s.Bytes()) == 0 {
			continue
		}
		var v trace2memv1.Event
		if e := protojson.Unmarshal(s.Bytes(), &v); e != nil {
			return fmt.Errorf("line %d: %w", line, e)
		}
		if e := emit(&v); e != nil {
			return fmt.Errorf("line %d: %w", line, e)
		}
	}
	return s.Err()
}
