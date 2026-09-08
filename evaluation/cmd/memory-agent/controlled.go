package main

import (
	"encoding/json"
	"strings"

	"github.com/trace2mem/trace2mem/internal/model"
)

func controlledTool() model.Tool {
	return model.Tool{Required: true, Name: "memory_step", Description: "Choose an action: read memory files, search memory, or submit an answer. Supply unused lists/strings as empty. Answers must cite original evidence. File-memory conditions require a read before answering and all cited evidence to have been read completely.", Parameters: map[string]any{
		"type": "object", "additionalProperties": false,
		"properties": map[string]any{
			"action":      map[string]any{"type": "string", "enum": []string{"read", "search", "answer"}},
			"paths":       map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
			"offset":      map[string]any{"type": "integer"},
			"query":       map[string]any{"type": "string"},
			"answer_json": map[string]any{"type": "string"},
		}, "required": []string{"action", "paths", "offset", "query", "answer_json"},
	}}
}

func controlledStep(condition string, pages map[string]string, call model.Call, read *bool, sources map[string]bool, coverage map[string]int) (any, map[string]any) {
	var args struct {
		Action string   `json:"action"`
		Paths  []string `json:"paths"`
		Offset int      `json:"offset"`
		Query  string   `json:"query"`
		Answer string   `json:"answer_json"`
	}
	bad := func(message string) (any, map[string]any) { return map[string]any{"error": message}, nil }
	if call.Name != "memory_step" || len(call.Arguments) > 16<<10 || json.Unmarshal(call.Arguments, &args) != nil {
		return bad("invalid action")
	}
	switch args.Action {
	case "read":
		if len(args.Paths) < 1 || len(args.Paths) > 4 {
			return bad("read 1 to 4 paths")
		}
		values := []any{}
		for _, path := range args.Paths {
			encoded, _ := json.Marshal(map[string]any{"path": path, "offset": args.Offset})
			value := execute(condition, pages, model.Call{Name: "memory_read", Arguments: encoded}).(map[string]any)
			if recordRead(value, coverage, sources) {
				*read = true
			}
			values = append(values, value)
		}
		return values, nil
	case "search":
		encoded, _ := json.Marshal(map[string]any{"query": args.Query})
		return execute(condition, pages, model.Call{Name: "memory_search", Arguments: encoded}), nil
	case "answer":
		if condition != "existing_memory" && !*read {
			return bad("read relevant memory before answering")
		}
		var artifact map[string]any
		if json.Unmarshal([]byte(args.Answer), &artifact) != nil || len(artifact) < 2 {
			return bad("answer_json must contain answer fields and citations")
		}
		citations, ok := artifact["citations"].(map[string]any)
		if !ok {
			return bad("citations must map fields to original event ID arrays")
		}
		for field := range artifact {
			if field == "citations" {
				continue
			}
			ids, ok := citations[field].([]any)
			if !ok || len(ids) == 0 {
				return bad("every field must cite original evidence, including explicit unknowns")
			}
			for _, id := range ids {
				name, ok := id.(string)
				if !ok {
					return bad("citation IDs must be strings")
				}
				if _, ok := pages["sessions/evidence/"+name+".json"]; !ok {
					return bad("citation does not exist in this revision")
				}
				if condition != "existing_memory" && !sources[name] {
					return bad("read every cited evidence file completely before submitting")
				}
			}
		}
		return map[string]any{"submitted": true}, artifact
	default:
		return bad("unknown action")
	}
}

func recordRead(value map[string]any, coverage map[string]int, sources map[string]bool) bool {
	if value["error"] != nil {
		return false
	}
	path, _ := value["path"].(string)
	if strings.HasPrefix(path, "sessions/evidence/") {
		offset, _ := value["offset"].(int)
		content, _ := value["content"].(string)
		if offset <= coverage[path] {
			coverage[path] = max(coverage[path], offset+len([]rune(content)))
			if value["next_offset"] == nil {
				sources[strings.TrimSuffix(strings.TrimPrefix(path, "sessions/evidence/"), ".json")] = true
			}
		}
	}
	return true
}
