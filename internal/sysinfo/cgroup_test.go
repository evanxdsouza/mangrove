package sysinfo

import "testing"

func TestVerifyCgroupV2(t *testing.T) {
	// This CI/dev environment should itself be running cgroup v2; if this
	// assertion starts failing it means the assumption baked into the
	// systemd slice files no longer holds for hosts we actually run on.
	if err := VerifyCgroupV2(); err != nil {
		t.Errorf("expected cgroup v2 on this host, got: %v", err)
	}
}
