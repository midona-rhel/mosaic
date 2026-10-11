package ui

import (
	"github.com/midona-rhel/mosaic/internal/metrics"
	"sort"
	"strings"
)

// Keep raw inventory in telemetry; the capacity UI presents usable storage.
func visibleVolumes(all []metrics.Volume) []metrics.Volume {
	var result []metrics.Volume
	seen := map[string]bool{}
	hasData := false
	for _, v := range all {
		if v.Mount == "/System/Volumes/Data" && v.Total > 0 {
			hasData = true
		}
	}
	for _, v := range all {
		mount := strings.ToLower(strings.ReplaceAll(v.Mount, "\\", "/"))
		kind := strings.ToLower(v.Type)
		if v.Total == 0 {
			continue
		}
		switch kind {
		case "devfs", "devtmpfs", "tmpfs", "proc", "procfs", "sysfs", "cgroup", "cgroup2", "autofs", "fdesc", "fdescfs", "debugfs", "securityfs", "tracefs", "pstore", "efivarfs", "mqueue", "hugetlbfs", "squashfs":
			continue
		}
		if mount == "/dev" || strings.HasPrefix(mount, "/dev/") || mount == "/proc" || strings.HasPrefix(mount, "/proc/") || mount == "/sys" || strings.HasPrefix(mount, "/sys/") || mount == "/boot" || strings.HasPrefix(mount, "/boot/") {
			continue
		}
		if strings.HasPrefix(mount, "/system/volumes/") && mount != "/system/volumes/data" {
			continue
		}
		// The APFS system snapshot and Data volume commonly describe the same pool.
		if hasData && v.Mount == "/" && kind == "apfs" {
			duplicate := false
			for _, d := range all {
				if d.Mount == "/System/Volumes/Data" && d.Total == v.Total && d.Free == v.Free {
					duplicate = true
					break
				}
			}
			if duplicate {
				continue
			}
		}
		key := v.Device + "|" + kind
		if v.Device != "" && seen[key] {
			continue
		}
		seen[key] = true
		result = append(result, v)
	}
	rank := func(v metrics.Volume) int {
		p := strings.ToLower(strings.ReplaceAll(v.Mount, "\\", "/"))
		if p == "/" || p == "/system/volumes/data" || p == "c:/" {
			return 0
		}
		return 1
	}
	sort.SliceStable(result, func(i, j int) bool {
		if rank(result[i]) != rank(result[j]) {
			return rank(result[i]) < rank(result[j])
		}
		return strings.ToLower(result[i].Mount) < strings.ToLower(result[j].Mount)
	})
	return result
}

func diskCardCount(h int) int {
	if h < 6 {
		return 1
	}
	return max(1, (h-1)/4)
}

func (m *Model) diskStart(panel, count int) int {
	return min(max(0, m.diskOffset[panel]), max(0, len(visibleVolumes(m.snapshot.Volumes))-count))
}
