package baraccess

import (
	"encoding/binary"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestReadNVMeExactAccesses(t *testing.T) {
	// Independent expected list: changing the policy must not silently expand
	// the regression test to include the same newly unsafe accesses.
	want := []int{0, 4, 8, 0x0c, 0x10, 0x14, 0x1c, 0x24, 0x28, 0x2c, 0x30, 0x34}
	var reads []int
	data, err := ReadNVMe(0, 0x10000, func(off int) (uint32, error) {
		reads = append(reads, off)
		return uint32(off + 1), nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(reads, want) || len(data) != 0x38 {
		t.Fatalf("reads=%#v, bytes=%d", reads, len(data))
	}
	for _, off := range want {
		if got := binary.LittleEndian.Uint32(data[off : off+4]); got != uint32(off+1) {
			t.Fatalf("offset %#x = %#x", off, got)
		}
	}
	for _, off := range []int{0x18, 0x20} {
		if binary.LittleEndian.Uint32(data[off:off+4]) != 0 {
			t.Fatalf("unread hole %#x was not zero padded", off)
		}
	}
}

func TestReadNVMeRejectsBeforeAccess(t *testing.T) {
	for _, tc := range []struct{ bar, size int }{
		{-1, 65536}, {1, 65536}, {2, 65536}, {5, 65536}, {0, -1}, {0, 0}, {0, 0x37},
	} {
		if _, err := ReadNVMe(tc.bar, tc.size, func(int) (uint32, error) {
			t.Fatal("must reject before touching MMIO")
			return 0, nil
		}); err == nil {
			t.Fatalf("accepted bar=%d size=%d", tc.bar, tc.size)
		}
	}
}

func TestReadNVMeStopsOnReadError(t *testing.T) {
	wantErr := errors.New("completion error")
	count := 0
	data, err := ReadNVMe(0, 4096, func(int) (uint32, error) {
		count++
		return 0, wantErr
	})
	if !errors.Is(err, wantErr) || data != nil || count != 1 {
		t.Fatalf("data=%v err=%v count=%d", data, err, count)
	}
}

func TestReadNVMeAllOnesCAPStopsBeforeVS(t *testing.T) {
	count := 0
	data, err := ReadNVMe(0, 65536, func(int) (uint32, error) {
		count++
		return ^uint32(0), nil
	})
	if err == nil || data != nil || count != 2 {
		t.Fatalf("data=%v err=%v reads=%d", data, err, count)
	}
}

func TestResourceClassFailsClosed(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "resource0")
	if _, err := ResourceIsNVMe(path); err == nil {
		t.Fatal("missing metadata accepted")
	}
	for _, tc := range []struct {
		class     string
		nvme, bad bool
	}{
		{"0x010802\n", true, false}, {"0x010800", false, false},
		{"0x020000", false, false}, {"garbage", false, true},
		{"0xffffff", false, true}, {"0x1010802", false, true},
	} {
		if err := os.WriteFile(filepath.Join(dir, "class"), []byte(tc.class), 0600); err != nil {
			t.Fatal(err)
		}
		got, err := ResourceIsNVMe(path)
		if got != tc.nvme || (err != nil) != tc.bad {
			t.Fatalf("class=%q got=%v err=%v", tc.class, got, err)
		}
	}
}

func TestLoad32BoundsAndWidth(t *testing.T) {
	words := []uint32{0x12345678, 0xabcdef01}
	// Use an aligned mapping surrogate, as the real mmap is page aligned.
	data := make([]byte, 8)
	for i, word := range words {
		binary.LittleEndian.PutUint32(data[i*4:], word)
		got, err := Load32(data, i*4)
		if err != nil || got != word {
			t.Fatalf("got=%#x err=%v", got, err)
		}
	}
	for _, off := range []int{-1, 1, 2, 6, 8} {
		if _, err := Load32(data, off); err == nil {
			t.Fatalf("accepted offset %d", off)
		}
	}
}
