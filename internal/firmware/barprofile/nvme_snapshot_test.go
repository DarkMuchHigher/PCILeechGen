package barprofile

import (
	"testing"

	"github.com/sercanarga/pcileechgen/internal/donor"
	"github.com/sercanarga/pcileechgen/internal/donor/baraccess"
	"github.com/sercanarga/pcileechgen/internal/pci"
)

func TestNVMeSnapshotDoesNotClaimMeasuredBehavior(t *testing.T) {
	p := &donor.BARProfile{ReadPolicy: baraccess.NVMeReadPolicy, Probes: []donor.BARProbeResult{{Offset: 0x14, Original: 0}, {Offset: 8, Original: 0x10300}}}
	if got := emulationHint(pci.BAR{Type: pci.BARTypeMem64}, make([]byte, 0x38), p); got != HintSpecModel {
		t.Fatalf("hint=%s", got)
	}
	for _, reg := range summariesFromProbe(p) {
		if reg.Kind != RegisterUnknown {
			t.Fatalf("unmeasured behavior classified as %s", reg.Kind)
		}
	}
}
