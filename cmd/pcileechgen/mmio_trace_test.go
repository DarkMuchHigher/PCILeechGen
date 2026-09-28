package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sercanarga/pcileechgen/internal/donor/vfio"
)

func TestMMIOTrace_RequiresLiveFlag(t *testing.T) {
	previous := mmioTraceOpts
	t.Cleanup(func() { mmioTraceOpts = previous })
	mmioTraceOpts = mmioTraceOptions{bdf: "0000:03:00.0", barSize: 4096, barBase: "0xf7800000", duration: time.Second}

	err := mmioTraceCmd.RunE(mmioTraceCmd, nil)
	if err == nil || !strings.Contains(err.Error(), "--live") {
		t.Fatalf("live capture without --live: err = %v, want refusal", err)
	}
}

func TestMMIOTrace_RejectsLiveHostEnvironment(t *testing.T) {
	dir := t.TempDir()
	mountInfo := filepath.Join(dir, "mountinfo")
	cmdline := filepath.Join(dir, "cmdline")
	if err := os.WriteFile(mountInfo, []byte("36 25 0:32 / / rw - overlay overlay rw\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cmdline, []byte("quiet splash boot=casper"), 0o644); err != nil {
		t.Fatal(err)
	}
	vfio.SetProcPaths(mountInfo, cmdline)
	t.Cleanup(vfio.ResetProcPaths)

	opts := mmioTraceOptions{bdf: "0000:03:00.0", barSize: 4096, live: true, duration: time.Second}
	if _, err := loadMMIOTrace(opts, 0xf7800000); err == nil || !strings.Contains(err.Error(), "live/USB-boot") {
		t.Fatalf("live capture on live host: err = %v, want refusal", err)
	}
}

func TestParseTraceBARBase(t *testing.T) {
	got, err := parseTraceBARBase("f7800000")
	if err != nil {
		t.Fatalf("parseTraceBARBase returned error: %v", err)
	}
	if got != 0xf7800000 {
		t.Fatalf("bar base = 0x%X, want 0xF7800000", got)
	}
}

func TestLoadMMIOTrace_FromTraceFile(t *testing.T) {
	tracePath := filepath.Join(t.TempDir(), "mmiotrace.txt")
	input := []byte("R 4 2456.105919 2 0xf780010c 0x4c02 0x0 0\n")
	if err := os.WriteFile(tracePath, input, 0o644); err != nil {
		t.Fatalf("write trace fixture: %v", err)
	}

	trace, err := loadMMIOTrace(mmioTraceOptions{
		bdf:       "0000:03:00.0",
		barIndex:  2,
		barSize:   4096,
		traceFile: tracePath,
	}, 0xf7800000)

	if err != nil {
		t.Fatalf("loadMMIOTrace returned error: %v", err)
	}
	if trace.BDF != "0000:03:00.0" || trace.BARIndex != 2 || trace.BARSize != 4096 {
		t.Fatalf("trace metadata = %+v", trace)
	}
	if len(trace.Records) != 1 || trace.Records[0].Offset != 0x10c || trace.Records[0].Value != 0x4c02 {
		t.Fatalf("records = %+v", trace.Records)
	}
}
