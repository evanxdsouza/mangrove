package sysinfo

import (
	"testing"
	"time"
)

func TestHostHealthSample(t *testing.T) {
	h := HostHealthSample(200 * time.Millisecond)

	if h.Hostname == "" {
		t.Error("expected non-empty hostname")
	}
	if h.CPUCount < 1 {
		t.Error("expected at least one CPU")
	}
	if h.CPUPercent < 0 || h.CPUPercent > 100 {
		t.Errorf("cpu_percent out of range: %v", h.CPUPercent)
	}
	if h.MemoryTotalGB <= 0 {
		t.Error("expected positive memory total")
	}
	if h.MemoryUsedGB <= 0 || h.MemoryUsedGB > h.MemoryTotalGB {
		t.Errorf("memory used %v out of range of total %v", h.MemoryUsedGB, h.MemoryTotalGB)
	}
	if h.DiskTotalGB <= 0 {
		t.Error("expected positive disk total")
	}
	if h.DiskUsedGB <= 0 || h.DiskUsedGB > h.DiskTotalGB {
		t.Errorf("disk used %v out of range of total %v", h.DiskUsedGB, h.DiskTotalGB)
	}
	if h.LoadAvg1 < 0 {
		t.Error("load average should be non-negative")
	}
	if h.Processes < 1 {
		t.Error("expected at least one process")
	}
	if h.UptimeSeconds <= 0 {
		t.Error("expected positive uptime")
	}
}