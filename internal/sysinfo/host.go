package sysinfo

import (
	"os"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// HostHealth is a point-in-time snapshot of the whole host, not just
// Mangrove's containers -- what the admin "Server health" page renders.
// All fields are read from procfs/sysfs so the single mangrove binary needs
// no external agent.
type HostHealth struct {
	Hostname       string  `json:"hostname"`
	Kernel         string  `json:"kernel"`
	UptimeSeconds  int64   `json:"uptime_seconds"`
	LoadAvg1       float64 `json:"load_avg_1"`
	LoadAvg5       float64 `json:"load_avg_5"`
	LoadAvg15      float64 `json:"load_avg_15"`
	CPUCount       int     `json:"cpu_count"`
	CPUPercent     float64 `json:"cpu_percent"`
	MemoryTotalGB  float64 `json:"memory_total_gb"`
	MemoryUsedGB   float64 `json:"memory_used_gb"`
	MemoryUsedPct  float64 `json:"memory_used_pct"`
	SwapTotalGB    float64 `json:"swap_total_gb"`
	SwapUsedGB     float64 `json:"swap_used_gb"`
	DiskTotalGB    float64 `json:"disk_total_gb"`
	DiskUsedGB     float64 `json:"disk_used_gb"`
	DiskUsedPct    float64 `json:"disk_used_pct"`
	Processes      int     `json:"processes"`
	RunningProcess int     `json:"running_process"`
}

// HostHealthSample gathers a HostHealth snapshot. CPU usage is measured by
// sampling /proc/stat idle-vs-total deltas across sampleFor; the other
// values are read once. Disk stats are reported for path (Mangrove's
// DataDir), the filesystem the host actually runs on.
func HostHealthSample(sampleFor time.Duration) HostHealth {
	h := HostHealth{}
	h.Hostname, _ = os.Hostname()
	h.Kernel = readKernel()
	h.UptimeSeconds, h.Processes, h.RunningProcess = readProcUptimeAndProc()
	h.LoadAvg1, h.LoadAvg5, h.LoadAvg15 = readLoadAvg()
	h.CPUCount = readCPUCount()
	h.CPUPercent = sampleCPUPercent(sampleFor)
	h.MemoryTotalGB, h.MemoryUsedGB, h.SwapTotalGB, h.SwapUsedGB = readMeminfo()
	if h.MemoryTotalGB > 0 {
		h.MemoryUsedPct = 100 * h.MemoryUsedGB / h.MemoryTotalGB
	}
	h.DiskTotalGB, h.DiskUsedGB = readDiskUsage("/")
	if h.DiskTotalGB > 0 {
		h.DiskUsedPct = 100 * h.DiskUsedGB / h.DiskTotalGB
	}
	return h
}

func readKernel() string {
	data, err := os.ReadFile("/proc/version")
	if err != nil {
		return ""
	}
	fields := strings.Fields(string(data))
	if len(fields) >= 3 {
		return fields[0] + " " + fields[2] // "Linux 6.x.y"
	}
	return strings.TrimSpace(string(data))
}

// readProcUptimeAndProc parses /proc/uptime (seconds) and /proc/loadavg's
// task-count fields, which /proc/meminfo can't provide. The loadavg line is
// "1 5 15 running/total pid"; uptime "sec sec2".
func readProcUptimeAndProc() (uptime int64, total, running int) {
	if data, err := os.ReadFile("/proc/uptime"); err == nil {
		if f := strings.Fields(string(data)); len(f) >= 1 {
			if v, err := strconv.ParseFloat(f[0], 64); err == nil {
				uptime = int64(v)
			}
		}
	}
	if data, err := os.ReadFile("/proc/loadavg"); err == nil {
		if fields := strings.Fields(string(data)); len(fields) >= 4 {
			if parts := strings.Split(fields[3], "/"); len(parts) == 2 {
				running, _ = strconv.Atoi(parts[0])
				total, _ = strconv.Atoi(parts[1])
			}
		}
	}
	return
}

func readLoadAvg() (l1, l5, l15 float64) {
	data, err := os.ReadFile("/proc/loadavg")
	if err != nil {
		return
	}
	fields := strings.Fields(string(data))
	if len(fields) < 3 {
		return
	}
	l1, _ = strconv.ParseFloat(fields[0], 64)
	l5, _ = strconv.ParseFloat(fields[1], 64)
	l15, _ = strconv.ParseFloat(fields[2], 64)
	return
}

func readCPUCount() int {
	data, err := os.ReadFile("/proc/stat")
	if err != nil {
		return 0
	}
	count := 0
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "cpu") && len(line) > 3 && line[3] != ' ' {
			count++
		}
	}
	return count
}

type cpuJiffies struct {
	user, nice, system, idle, iowait, irq, softirq, steal uint64
}

func readCPUTicks() (total, idle uint64) {
	data, err := os.ReadFile("/proc/stat")
	if err != nil {
		return
	}
	for _, line := range strings.Split(string(data), "\n") {
		if !strings.HasPrefix(line, "cpu ") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 8 {
			return
		}
		var j cpuJiffies
		vals := []*uint64{&j.user, &j.nice, &j.system, &j.idle, &j.iowait, &j.irq, &j.softirq, &j.steal}
		for i, v := range vals {
			*v, _ = strconv.ParseUint(fields[i+1], 10, 64)
		}
		idle = j.idle + j.iowait
		total = j.user + j.nice + j.system + j.idle + j.iowait + j.irq + j.softirq + j.steal
		return
	}
	return
}

// sampleCPUPercent measures CPU usage across sampleFor by diffing /proc/stat
// busy-vs-idle jiffies before and after, the standard procfs technique.
func sampleCPUPercent(sampleFor time.Duration) float64 {
	t0, i0 := readCPUTicks()
	time.Sleep(sampleFor)
	t1, i1 := readCPUTicks()
	dt := t1 - t0
	di := i1 - i0
	if dt == 0 || t0 == 0 {
		return 0
	}
	used := float64(dt-di) / float64(dt) * 100
	if used < 0 {
		return 0
	}
	if used > 100 {
		return 100
	}
	return used
}

// readMeminfo parses /proc/meminfo's MemTotal/MemAvailable (kB) and the
// swap equivalents into GB. MemAvailable is the kernel's own estimate of
// memory that can be handed out -- far closer to "how much is actually in
// use" than subtracting buffers/cache manually.
func readMeminfo() (memTotal, memUsed, swapTotal, swapUsed float64) {
	data, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return
	}
	kb := map[string]float64{}
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		key := strings.TrimSuffix(fields[0], ":")
		v, err := strconv.ParseFloat(fields[1], 64)
		if err != nil {
			continue
		}
		kb[key] = v
	}
	const gb = 1024 * 1024
	memTotal = kb["MemTotal"] / gb
	if avail, ok := kb["MemAvailable"]; ok {
		memUsed = (kb["MemTotal"] - avail) / gb
	} else {
		memUsed = kb["MemTotal"] / gb // fallback: no available counter, report full
	}
	swapTotal = kb["SwapTotal"] / gb
	swapUsed = (kb["SwapTotal"] - kb["SwapFree"]) / gb
	return
}

// readDiskUsage stats the filesystem containing path and returns total/used
// in GB.
func readDiskUsage(path string) (total, used float64) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		return
	}
	const gb = 1024 * 1024 * 1024
	blockSize := float64(stat.Bsize)
	total = float64(stat.Blocks) * blockSize / gb
	used = (float64(stat.Blocks) - float64(stat.Bfree)) * blockSize / gb
	return
}