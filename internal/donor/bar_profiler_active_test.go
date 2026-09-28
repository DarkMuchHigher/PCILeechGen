package donor

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPublicProfilerNeverWritesToBAR(t *testing.T) {
	dir := t.TempDir()
	res := filepath.Join(dir, "resource0")
	if err := os.WriteFile(filepath.Join(dir, "class"), []byte("0x020000\n"), 0600); err != nil {
		t.Fatal(err)
	}
	original := make([]byte, 4096)
	for i := range original {
		original[i] = byte(i)
	}
	if err := os.WriteFile(res, original, 0600); err != nil {
		t.Fatal(err)
	}

	profile, err := NewBARProfiler().ProfileBAR(res, 0, 4096)
	if err != nil {
		t.Fatalf("read-only profiling failed: %v", err)
	}
	if profile == nil || len(profile.Probes) == 0 {
		t.Fatal("read-only profiling must still snapshot registers")
	}
	for _, p := range profile.Probes {
		if p.RWMask != 0 || p.W1CMask != 0 {
			t.Fatalf("read-only probe reported write masks: %+v", p)
		}
	}

	after, err := os.ReadFile(res)
	if err != nil {
		t.Fatal(err)
	}
	for i := range after {
		if after[i] != original[i] {
			t.Fatalf("BAR content modified at %#x via the public constructor", i)
		}
	}
}
