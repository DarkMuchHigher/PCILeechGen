package donor

import (
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"

	"github.com/sercanarga/pcileechgen/internal/donor/baraccess"
	"github.com/sercanarga/pcileechgen/internal/pci"
)

func nvmeBARFixture(t *testing.T) (*Collector, pci.BDF, string, []byte) {
	t.Helper()
	base := createMockSysfs(t)
	bdf := pci.BDF{Bus: 3}
	dir := filepath.Join(base, bdf.String())
	writeFile(t, dir, "class", "0x010802\n")
	data := bytes.Repeat([]byte{0xa5}, 65536)
	binary.LittleEndian.PutUint64(data[:8], 0x000000200040ff17)
	binary.LittleEndian.PutUint32(data[8:12], 0x00010300)
	if err := os.WriteFile(filepath.Join(dir, "resource0"), data, 0600); err != nil {
		t.Fatal(err)
	}
	return NewCollectorWithSysfs(NewSysfsReaderWithPath(base)), bdf, dir, data
}

func checkNVMeSnapshot(t *testing.T, got, original []byte) {
	t.Helper()
	if len(got) != 0x38 {
		t.Fatalf("snapshot length=%d want 56", len(got))
	}
	for _, off := range baraccess.NVMeOffsets() {
		if !bytes.Equal(got[off:off+4], original[off:off+4]) {
			t.Fatalf("register %#x not preserved", off)
		}
	}
	if binary.LittleEndian.Uint32(got[0x18:]) != 0 || binary.LittleEndian.Uint32(got[0x20:]) != 0 {
		t.Fatal("reserved/NSSR holes were read")
	}
}

func TestNVMeAllSysfsReadEntrypoints(t *testing.T) {
	c, bdf, _, original := nvmeBARFixture(t)
	data, err := c.sysfs.ReadBARContent(bdf, 0, 65536)
	if err != nil {
		t.Fatal(err)
	}
	checkNVMeSnapshot(t, data, original)
	// Native-driver capture uses this same retry helper, so the read cap must
	// also survive the native-driver path, not just initial collection.
	data, err = c.readBARUntilValid(bdf, 0, 65536)
	if err != nil {
		t.Fatal(err)
	}
	checkNVMeSnapshot(t, data, original)
	contents := make(map[int][]byte)
	bars := []pci.BAR{{Index: 0, Size: 65536, Type: pci.BARTypeMem64}, {Index: 2, Size: 4096, Type: pci.BARTypeMem32}}
	c.readBARs(bdf, bars, contents)
	checkNVMeSnapshot(t, contents[0], original)
	if _, ok := contents[2]; ok {
		t.Fatal("other NVMe BAR was captured")
	}
}

func TestNVMeProfilerNoWritesOrDoorbells(t *testing.T) {
	_, _, dir, original := nvmeBARFixture(t)
	path := filepath.Join(dir, "resource0")
	profile, err := NewBARProfiler().ProfileBAR(path, 0, 65536)
	if err != nil {
		t.Fatal(err)
	}
	if profile.Size != 0x38 || len(profile.Probes) != 12 || profile.ReadPolicy == "" {
		t.Fatalf("unexpected profile: %+v", profile)
	}
	for i, off := range baraccess.NVMeOffsets() {
		p := profile.Probes[i]
		if p.Offset != uint32(off) || p.Original != binary.LittleEndian.Uint32(original[off:]) || p.RWMask != 0 || p.W1CMask != 0 {
			t.Fatalf("unexpected probe: %+v", p)
		}
	}
	if _, err := NewActiveBARProfiler().ProfileBAR(path, 0, 65536); err == nil {
		t.Fatal("active NVMe probing permitted")
	}
	if _, err := NewBARProfiler().ProfileBAR(filepath.Join(dir, "resource2"), 2, 65536); err == nil {
		t.Fatal("other NVMe BAR profiling permitted")
	}
	after, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(after, original) {
		t.Fatal("profiler changed the resource")
	}
}

func TestNVMeProfilesUseRAMOnly(t *testing.T) {
	// No sysfs reader at all: profiling cannot reopen hardware, including after
	// a missing/failed BAR snapshot. A large old buffer must not expand policy.
	c := &Collector{}
	bars := []pci.BAR{{Index: 0, Size: 65536, Type: pci.BARTypeMem64}, {Index: 2, Size: 4096, Type: pci.BARTypeMem32}}
	contents := map[int][]byte{0: bytes.Repeat([]byte{1}, 65536)}
	profiles := c.collectBARProfiles(0x010802, bars, contents)
	if len(profiles) != 1 || len(profiles[0].Probes) != 12 || profiles[0].Size != 0x38 {
		t.Fatalf("unexpected profiles: %+v", profiles)
	}
	if got := c.collectBARProfiles(0x010802, bars, nil); len(got) != 0 {
		t.Fatal("profile produced without snapshot")
	}
}

func TestNVMeRejectsShortBARAndAllOnes(t *testing.T) {
	c, bdf, dir, _ := nvmeBARFixture(t)
	for _, size := range []int{-1, 0, 0x37} {
		if _, err := c.sysfs.ReadBARContent(bdf, 0, size); err == nil {
			t.Fatalf("accepted maxSize=%d", size)
		}
	}
	for _, data := range [][]byte{make([]byte, 0x37), bytes.Repeat([]byte{0xff}, 65536)} {
		if err := os.WriteFile(filepath.Join(dir, "resource0"), data, 0600); err != nil {
			t.Fatal(err)
		}
		if got, err := c.sysfs.ReadBARContent(bdf, 0, 65536); err == nil || got != nil {
			t.Fatal("invalid BAR snapshot accepted (zero holes must not hide all-ones CAP)")
		}
	}
}
