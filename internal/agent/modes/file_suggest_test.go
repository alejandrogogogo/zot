package modes

import (
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"
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

// TestFileSuggesterPicksUpNewEntries pins the cache-invalidation bug
// fix: creating a subdirectory after the picker has already scanned
// the cwd must surface that subdirectory on the next scan, without
// any explicit invalidation call. The cache is keyed on the dir's
// mtime, which the OS bumps on every entry add/remove/rename.
func TestFileSuggesterPicksUpNewEntries(t *testing.T) {
	tmp := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmp, "existing.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	s := newFileSuggester()
	s.SetCWD(tmp)

	first := s.scan()
	if !containsEntry(first, "existing.txt", false) {
		t.Fatalf("first scan missing existing.txt: %#v", first)
	}
	if containsEntry(first, "test", true) {
		t.Fatalf("first scan unexpectedly saw the not-yet-created test/: %#v", first)
	}

	// Sleep one filesystem tick: HFS+/APFS/ext4 all bump directory
	// mtime with at-least 1s resolution depending on mount options.
	// Without this sleep on coarse-resolution filesystems the mtime
	// after Mkdir can equal the mtime captured during the first scan
	// and the cache would (correctly) be retained.
	time.Sleep(1100 * time.Millisecond)
	if err := os.Mkdir(filepath.Join(tmp, "test"), 0o755); err != nil {
		t.Fatal(err)
	}

	second := s.scan()
	if !containsEntry(second, "test", true) {
		t.Fatalf("second scan did not pick up the newly created test/: %#v", second)
	}
	if !containsEntry(second, "existing.txt", false) {
		t.Fatalf("second scan dropped existing.txt: %#v", second)
	}

	// Directories sort before files.
	sorted := make([]string, 0, len(second))
	for _, e := range second {
		sorted = append(sorted, e.name)
	}
	if !sort.IsSorted(byDirsFirst(second)) {
		t.Fatalf("entries are not dirs-first / case-insensitive sorted: %v", sorted)
	}
}

func containsEntry(entries []fileEntry, name string, isDir bool) bool {
	for _, e := range entries {
		if e.name == name && e.isDir == isDir {
			return true
		}
	}
	return false
}

type byDirsFirst []fileEntry

func (b byDirsFirst) Len() int      { return len(b) }
func (b byDirsFirst) Swap(i, j int) { b[i], b[j] = b[j], b[i] }
func (b byDirsFirst) Less(i, j int) bool {
	if b[i].isDir != b[j].isDir {
		return b[i].isDir
	}
	return b[i].name < b[j].name
}
