// Package sysinfo checks host-level preconditions the resource-floor
// protection in deploy/systemd/*.slice depends on.
package sysinfo

import (
	"fmt"
	"os"
)

// VerifyCgroupV2 confirms the host uses the unified cgroup v2 hierarchy,
// which mangrove.slice / mangrove-deployments.slice's MemoryMin/MemoryMax
// depend on. cgroup v1 (or a v1/v2 hybrid) silently ignores MemoryMin, so
// callers should refuse to claim the reserved-floor protection is active
// rather than pretend it's enforced when it isn't.
func VerifyCgroupV2() error {
	// cgroup v2's single unified hierarchy is mounted at /sys/fs/cgroup and
	// contains a "cgroup.controllers" file; v1 and hybrid setups don't
	// expose this at the root.
	if _, err := os.Stat("/sys/fs/cgroup/cgroup.controllers"); err != nil {
		return fmt.Errorf("cgroup v2 unified hierarchy not detected (/sys/fs/cgroup/cgroup.controllers missing) — " +
			"the mangrove.slice/mangrove-deployments.slice reserved-floor protection requires cgroup v2 and will not " +
			"take effect on this host; see deploy/systemd/README")
	}
	return nil
}
