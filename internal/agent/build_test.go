package agent

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadAgentsContextLoadsGlobalAndAncestors(t *testing.T) {
	root := t.TempDir()
	zotHome := filepath.Join(root, "zot-home")
	project := filepath.Join(root, "repo")
	nested := filepath.Join(project, "packages", "app")
	if err := os.MkdirAll(zotHome, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(zotHome, "AGENTS.md"), []byte("global rule"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(project, "AGENTS.md"), []byte("repo rule"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nested, "AGENTS.md"), []byte("app rule"), 0o644); err != nil {
		t.Fatal(err)
	}

	got := readAgentsContext(nested, zotHome)
	for _, want := range []string{"global rule", "repo rule", "app rule"} {
		if !strings.Contains(got, want) {
			t.Fatalf("readAgentsContext missing %q in:\n%s", want, got)
		}
	}
	if strings.Index(got, "global rule") > strings.Index(got, "repo rule") || strings.Index(got, "repo rule") > strings.Index(got, "app rule") {
		t.Fatalf("AGENTS.md files loaded in wrong order:\n%s", got)
	}
}

func TestReadAgentsContextMissingFilesIsEmpty(t *testing.T) {
	got := readAgentsContext(t.TempDir(), t.TempDir())
	if got != "" {
		t.Fatalf("expected no context, got %q", got)
	}
}
