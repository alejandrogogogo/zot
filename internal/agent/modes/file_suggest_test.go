package modes

import (
	"path/filepath"
	"testing"
)

func TestExpandFileChipsAtPickerShape(t *testing.T) {
	cwd := filepath.Join(string(filepath.Separator), "repo")
	in := "read [file:README.md] and [dir:docs/]"
	want := "read " + filepath.Join(cwd, "README.md") + " and " + filepath.Join(cwd, "docs")
	if got := expandFileChips(in, cwd); got != want {
		t.Fatalf("expandFileChips() = %q, want %q", got, want)
	}
}

func TestExpandFileChipsLeavesEditorPlaceholderShapeAlone(t *testing.T) {
	cwd := filepath.Join(string(filepath.Separator), "repo")
	// tui.Editor.SubmitValue should expand [file:N:name] using its
	// private path map before modes see the text. If such a token leaks
	// through, modes must not guess that "1:foo.txt" is a relative path.
	in := "read [file:1:foo.txt]"
	if got := expandFileChips(in, cwd); got != in {
		t.Fatalf("editor placeholder was changed: %q", got)
	}
}
