package tui

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/patriceckhart/zot/packages/provider"
)

func TestLiveToolOverlayRemainsAfterAssistantToolUse(t *testing.T) {
	args := json.RawMessage(`{"command":"sleep 1"}`)
	v := View{
		Theme: Dark,
		Messages: []provider.Message{
			{
				Role: provider.RoleAssistant,
				Content: []provider.Content{
					provider.ToolCallBlock{ID: "toolu_1", Name: "bash", Arguments: args},
				},
			},
		},
		ToolCalls: []ToolCallView{
			{ID: "toolu_1", Name: "bash", Args: ShortArgs("bash", args), Done: false},
		},
	}

	plain := stripANSI(strings.Join(v.Build(80), "\n"))
	if !strings.Contains(plain, "bash sleep 1") {
		t.Fatalf("live tool overlay disappeared after assistant tool_use was appended:\n%s", plain)
	}
}

func TestLiveToolOverlayShowsFullBashCommandBeforeResult(t *testing.T) {
	args := json.RawMessage(`{"command":"printf 'start' && sleep 60 && printf 'done with a command long enough to exceed the header truncation limit'"}`)
	v := View{
		Theme: Dark,
		ToolCalls: []ToolCallView{
			{
				ID:         "toolu_1",
				Name:       "bash",
				Args:       ShortArgs("bash", args),
				RawJSONBuf: string(args),
			},
		},
	}

	plain := stripANSI(strings.Join(v.BuildLive(100), "\n"))
	if !strings.Contains(plain, "$ printf 'start' && sleep 60") || !strings.Contains(plain, "header truncation limit") {
		t.Fatalf("bash command was not visible before result arrived:\n%s", plain)
	}
}

func TestLiveToolOverlayHeightDoesNotShrink(t *testing.T) {
	tall := json.RawMessage(`{"command":"line1\nline2\nline3\nline4\nline5"}`)
	v := View{
		Theme: Dark,
		ToolCalls: []ToolCallView{
			{ID: "toolu_1", Name: "bash", Args: ShortArgs("bash", tall), RawJSONBuf: string(tall)},
		},
	}
	tallRows := len(v.BuildLive(80))
	if tallRows == 0 {
		t.Fatal("expected rows for the tall command")
	}

	// Reserve the observed height, then switch to a shorter command
	// (e.g. the next phase of the same turn). The overlay must stay at
	// least as tall as before so the editor/status area never jumps up.
	v.LiveToolMinRows = tallRows
	short := json.RawMessage(`{"command":"echo hi"}`)
	v.ToolCalls = []ToolCallView{
		{ID: "toolu_2", Name: "bash", Args: ShortArgs("bash", short), RawJSONBuf: string(short)},
	}
	shortBuild := v.BuildLive(80)
	if got := len(shortBuild); got < tallRows {
		t.Fatalf("live overlay shrank from %d to %d rows", tallRows, got)
	}
	// The interactive caller strips trailing blank rows; the
	// reservation must survive that trim, so the pad rows have to be
	// non-blank box rows rather than trailing blanks.
	trimmed := shortBuild
	for len(trimmed) > 0 && strings.TrimSpace(stripANSI(trimmed[len(trimmed)-1])) == "" {
		trimmed = trimmed[:len(trimmed)-1]
	}
	if len(trimmed) < tallRows {
		t.Fatalf("reservation lost to trailing-blank trim: %d rows after trim, want >= %d", len(trimmed), tallRows)
	}
}

func TestLiveToolOverlayKeepsWritePreviewAfterArgsEnd(t *testing.T) {
	args := json.RawMessage(`{"path":"/tmp/sample.ts","content":"export const n = 1\n"}`)
	v := View{
		Theme: Dark,
		Messages: []provider.Message{
			{
				Role: provider.RoleAssistant,
				Content: []provider.Content{
					provider.ToolCallBlock{ID: "toolu_1", Name: "write", Arguments: args},
				},
			},
		},
		ToolCalls: []ToolCallView{
			{
				ID:         "toolu_1",
				Name:       "write",
				Args:       ShortArgs("write", args),
				Streaming:  false,
				RawJSONBuf: string(args),
				LivePath:   "/tmp/sample.ts",
			},
		},
	}

	plain := stripANSI(strings.Join(v.Build(80), "\n"))
	if !strings.Contains(plain, "export const n = 1") {
		t.Fatalf("write preview collapsed after tool args ended but before tool_result arrived:\n%s", plain)
	}
}

func TestLiveToolOverlayHidesAfterToolResult(t *testing.T) {
	args := json.RawMessage(`{"command":"sleep 1"}`)
	v := View{
		Theme: Dark,
		Messages: []provider.Message{
			{
				Role: provider.RoleAssistant,
				Content: []provider.Content{
					provider.ToolCallBlock{ID: "toolu_1", Name: "bash", Arguments: args},
				},
			},
			{
				Role: provider.RoleTool,
				Content: []provider.Content{
					provider.ToolResultBlock{
						CallID:  "toolu_1",
						Content: []provider.Content{provider.TextBlock{Text: "done"}},
					},
				},
			},
		},
		ToolCalls: []ToolCallView{
			{ID: "toolu_1", Name: "bash", Args: ShortArgs("bash", args), Result: "done", Done: true},
		},
	}

	plain := stripANSI(strings.Join(v.BuildLive(80), "\n"))
	if strings.Contains(plain, "bash sleep 1") {
		t.Fatalf("live tool overlay still rendered after tool_result was appended:\n%s", plain)
	}
}
