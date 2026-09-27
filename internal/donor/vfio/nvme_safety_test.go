package vfio

import (
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
)

func TestReadNVMeRegionUsesOffsetAndWhitelist(t *testing.T) {
	path := filepath.Join(t.TempDir(), "device")
	data := bytes.Repeat([]byte{0xa5}, 0x20000)
	const base = 0x10000
	binary.LittleEndian.PutUint64(data[base:], 0x000000200040ff17)
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	got, err := readNVMeRegion(int(f.Fd()), &vfioRegionInfo{Index: 0, Offset: base, Size: 65536})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0x38 || !bytes.Equal(got[:8], data[base:base+8]) || binary.LittleEndian.Uint32(got[0x18:]) != 0 || binary.LittleEndian.Uint32(got[0x20:]) != 0 {
		t.Fatalf("unexpected snapshot: %x", got)
	}
	// Invalid fd demonstrates other BARs/short regions are rejected without I/O.
	for _, info := range []vfioRegionInfo{{Index: 2, Size: 65536}, {Index: 0, Size: 0x37}} {
		if _, err := readNVMeRegion(-1, &info); err == nil {
			t.Fatal("invalid region accepted")
		}
	}
	if _, err := readNVMeRegion(int(f.Fd()), &vfioRegionInfo{Index: 0, Offset: uint64(len(data) - 2), Size: 65536}); err == nil {
		t.Fatal("short pread accepted")
	}
}
