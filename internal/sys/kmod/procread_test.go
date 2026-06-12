package kmod

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The procfs truncate-at-512 behavior of awg_proxy.ko < 1.1.11 cannot be
// reproduced with regular files; these tests pin the helper's contract:
// content well past os.ReadFile's initial 512-byte chunk comes back whole
// from a single read.
func TestReadProcFullContent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "list")

	// 7 slot lines ≈ 730 bytes — past the 512-byte boundary from issue #362.
	line := "94.143.100.170:7144 listen=127.0.0.1:49842 rx=918224807 tx=61319740 rx_pkt=804503 tx_pkt=254365\n"
	content := []byte(strings.Repeat(line, 7))
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := ReadProc(path)
	if err != nil {
		t.Fatalf("ReadProc: %v", err)
	}
	if !bytes.Equal(got, content) {
		t.Errorf("ReadProc returned %d bytes, want %d", len(got), len(content))
	}
}

func TestReadProcEmptyFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "empty")
	if err := os.WriteFile(path, nil, 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := ReadProc(path)
	if err != nil {
		t.Fatalf("ReadProc: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("ReadProc returned %d bytes, want 0", len(got))
	}
}

func TestReadProcMissingFile(t *testing.T) {
	if _, err := ReadProc(filepath.Join(t.TempDir(), "nope")); err == nil {
		t.Error("ReadProc on missing file: want error, got nil")
	}
}
