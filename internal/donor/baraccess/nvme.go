package baraccess

import (
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync/atomic"
	"unsafe"

	"github.com/sercanarga/pcileechgen/internal/firmware/devclass"
)

const NVMeSnapshotSize = 0x38
const NVMeReadPolicy = "nvme-baseline-dword-v1"

func NVMeOffsets() []int {
	return []int{0x00, 0x04, 0x08, 0x0c, 0x10, 0x14, 0x1c, 0x24, 0x28, 0x2c, 0x30, 0x34}
}

func ResourceIsNVMe(resourcePath string) (bool, error) {
	raw, err := os.ReadFile(filepath.Join(filepath.Dir(resourcePath), "class"))
	if err != nil {
		return false, fmt.Errorf("BAR access requires PCI class metadata: %w", err)
	}
	class, err := strconv.ParseUint(strings.TrimSpace(string(raw)), 0, 24)
	if err != nil || class == 0xffffff {
		return false, fmt.Errorf("invalid PCI class metadata %q", strings.TrimSpace(string(raw)))
	}
	return devclass.IsNVMe(uint32(class)), nil
}

func ReadNVMe(barIndex, size int, read32 func(int) (uint32, error)) ([]byte, error) {
	if barIndex != 0 {
		return nil, fmt.Errorf("NVMe BAR%d skipped: only baseline BAR0 registers are allowlisted", barIndex)
	}
	if size < NVMeSnapshotSize {
		return nil, fmt.Errorf("NVMe BAR0 snapshot requires at least %#x bytes, got %d", NVMeSnapshotSize, size)
	}
	if read32 == nil {
		return nil, fmt.Errorf("NVMe BAR0 requires a DWORD reader")
	}
	data := make([]byte, NVMeSnapshotSize)
	for _, off := range NVMeOffsets() {
		value, err := read32(off)
		if err != nil {
			return nil, fmt.Errorf("NVMe BAR0 read at %#x: %w", off, err)
		}
		binary.LittleEndian.PutUint32(data[off:off+4], value)
		if off == 4 && binary.LittleEndian.Uint64(data[:8]) == ^uint64(0) {
			return nil, fmt.Errorf("NVMe BAR0 CAP is all ones; refusing further MMIO reads")
		}
	}
	return data, nil
}

func Load32(mapped []byte, off int) (uint32, error) {
	if runtime.GOARCH != "amd64" && runtime.GOARCH != "386" {
		return 0, fmt.Errorf("NVMe mmap DWORD access is unsupported on %s", runtime.GOARCH)
	}
	if off < 0 || off > len(mapped)-4 || off%4 != 0 {
		return 0, fmt.Errorf("invalid MMIO DWORD offset %#x", off)
	}
	ptr := unsafe.Pointer(&mapped[off])
	if uintptr(ptr)%4 != 0 {
		return 0, fmt.Errorf("unaligned MMIO mapping")
	}
	return atomic.LoadUint32((*uint32)(ptr)), nil
}
