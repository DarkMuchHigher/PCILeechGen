package devicemodel

import (
	"testing"

	"github.com/sercanarga/pcileechgen/internal/donor"
	"github.com/sercanarga/pcileechgen/internal/donor/baraccess"
	"github.com/sercanarga/pcileechgen/internal/pci"
)

func TestNVMeSnapshotAccessIsInferredFromSpec(t *testing.T) {
	ctx := &donor.DeviceContext{
		Device: pci.PCIDevice{ClassCode: 0x010802}, ConfigSpace: pci.NewConfigSpace(),
		BARContents: map[int][]byte{0: make([]byte, 0x38)},
		BARProfiles: map[int]*donor.BARProfile{0: {
			ReadPolicy: baraccess.NVMeReadPolicy,
			Probes:     []donor.BARProbeResult{{Offset: 0x14}},
		}},
	}
	found := false
	for _, reg := range buildRegisters(ctx) {
		if reg.Space != SpaceBAR {
			continue
		}
		found = true
		if reg.Confidence != ConfidenceInferred {
			t.Fatal("read-only snapshot claimed measured access behavior")
		}
		var rw uint64
		for _, field := range reg.Fields {
			if field.Access == AccessRW {
				rw |= field.Mask
			}
		}
		if rw != 0x00fffff1 {
			t.Fatalf("CC write mask=%#x", rw)
		}
	}
	if !found || buildConfidence(ctx).Overall == ConfidenceMeasured {
		t.Fatal("missing register or overstated model confidence")
	}
}
