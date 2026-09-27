package barmodel

import (
	"encoding/binary"
	"testing"

	"github.com/sercanarga/pcileechgen/internal/donor"
	"github.com/sercanarga/pcileechgen/internal/donor/baraccess"
	"github.com/sercanarga/pcileechgen/internal/pci"
)

func TestNVMeReadOnlySnapshotKeepsSpecWritesAndAperture(t *testing.T) {
	data := make([]byte, baraccess.NVMeSnapshotSize)
	binary.LittleEndian.PutUint64(data[:8], 0x000000200040ff17)
	binary.LittleEndian.PutUint32(data[8:12], 0x00010300)
	p := &donor.BARProfile{BarIndex: 0, Size: len(data), ReadPolicy: baraccess.NVMeReadPolicy}
	for _, off := range baraccess.NVMeOffsets() {
		p.Probes = append(p.Probes, donor.BARProbeResult{Offset: uint32(off), Original: binary.LittleEndian.Uint32(data[off:])})
	}
	models, err := BuildBARModels([]pci.BAR{{Index: 0, Size: 65536, Type: pci.BARTypeMem64}},
		map[int][]byte{0: data}, map[int]*donor.BARProfile{0: p}, 0x010802, 0)
	if err != nil {
		t.Fatal(err)
	}
	m := ModelForBIR(models, 0)
	if m == nil || m.Size != 65536 || m.Aperture != 65536 {
		t.Fatalf("lost hardware BAR aperture: %+v", m)
	}
	want := map[uint32]uint32{0x14: 0x00fffff1, 0x24: 0x0fff0fff, 0x28: 0xfffff000, 0x2c: 0xffffffff, 0x30: 0xfffff000, 0x34: 0xffffffff}
	for _, reg := range m.Registers {
		if mask, ok := want[reg.Offset]; ok {
			if reg.RWMask != mask {
				t.Fatalf("offset %#x mask=%#x want=%#x", reg.Offset, reg.RWMask, mask)
			}
			delete(want, reg.Offset)
		}
		if reg.Offset == 8 && reg.Reset != 0x00010300 {
			t.Fatal("lost captured NVMe version")
		}
	}
	if len(want) != 0 {
		t.Fatalf("zero-valued but required registers dropped: %v", want)
	}
}
