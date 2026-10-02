package stats

import (
	"errors"
	"os"
	"strconv"
	"strings"
)

// cpuTimes are the CPU times of the machine since it started, in ticks, from /proc/stat.
type cpuTimes struct {
	busy, total uint64
	cores       uint32
}

func readCPU() (cpuTimes, error) {
	data, err := os.ReadFile("/proc/stat")
	if err != nil {
		return cpuTimes{}, err
	}
	var t cpuTimes
	for line := range strings.Lines(string(data)) {
		fields := strings.Fields(line)
		switch {
		case len(fields) == 0:
		case len(fields) > 4 && fields[0] == "cpu":
			// user nice system idle iowait irq softirq steal; guest time is part of user
			for i, f := range fields[1:min(len(fields), 9)] {
				n, _ := strconv.ParseUint(f, 10, 64)
				t.total += n
				if i != 3 && i != 4 { // idle and iowait
					t.busy += n
				}
			}
		case strings.HasPrefix(fields[0], "cpu"):
			t.cores++
		}
	}
	if t.total == 0 {
		return t, errors.New("no CPU times in /proc/stat")
	}
	return t, nil
}

// readMemory returns the memory of the machine and how much of it is used, which is what
// applications can't get without swapping, from /proc/meminfo.
func readMemory() (used, total uint64, err error) {
	data, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return 0, 0, err
	}
	var available uint64
	for line := range strings.Lines(string(data)) {
		key, value, _ := strings.Cut(line, ":")
		kb, _ := strconv.ParseUint(strings.TrimSuffix(strings.TrimSpace(value), " kB"), 10, 64)
		switch key {
		case "MemTotal":
			total = kb << 10
		case "MemAvailable":
			available = kb << 10
		}
	}
	return total - min(available, total), total, nil
}
