package discoveryceiling

import (
	"os"
	"path/filepath"
	"testing"
)

func TestForUnsetIsInert(t *testing.T) {
	t.Setenv(Env, "")
	if got := For(t.TempDir()); got != "" {
		t.Fatalf("For with %s unset = %q, want empty", Env, got)
	}
}

func TestForAppliesInsideTheCeiling(t *testing.T) {
	root := t.TempDir()
	start := filepath.Join(root, "tmp", "case")
	if err := os.MkdirAll(start, 0o750); err != nil {
		t.Fatal(err)
	}
	t.Setenv(Env, root)

	want := canonical(root)
	for _, from := range []string{root, start} {
		if got := For(from); got != want {
			t.Errorf("For(%q) = %q, want %q", from, got, want)
		}
	}
	if !Reached(root, want) {
		t.Errorf("Reached(%q, %q) = false, want true", root, want)
	}
	if Reached(start, want) {
		t.Errorf("Reached(%q, %q) = true, want false", start, want)
	}
}

func TestForMatchesThroughASymlinkedAlias(t *testing.T) {
	base := t.TempDir()
	real := filepath.Join(base, "real")
	if err := os.MkdirAll(filepath.Join(real, "case"), 0o750); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(base, "alias")
	if err := os.Symlink(real, alias); err != nil {
		t.Fatal(err)
	}
	t.Setenv(Env, alias)

	ceiling := For(filepath.Join(real, "case"))
	if ceiling == "" {
		t.Fatalf("For(%q) = empty; a walk inside the symlink target must see the ceiling", filepath.Join(real, "case"))
	}
	if !Reached(real, ceiling) {
		t.Errorf("Reached(%q, %q) = false; the resolved directory is the ceiling", real, ceiling)
	}
}

func TestForIgnoresWalksOutsideTheCeiling(t *testing.T) {
	base := t.TempDir()
	ceiling := filepath.Join(base, "sandbox")
	outside := filepath.Join(base, "checkout")
	for _, dir := range []string{ceiling, outside} {
		if err := os.MkdirAll(dir, 0o750); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv(Env, ceiling)

	if got := For(outside); got != "" {
		t.Fatalf("For(%q) = %q, want empty for a walk outside the ceiling", outside, got)
	}
	if Reached(base, "") {
		t.Fatal("Reached with an empty ceiling = true, want false")
	}
}
