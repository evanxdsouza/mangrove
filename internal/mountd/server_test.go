package mountd

import (
	"encoding/json"
	"testing"
)

// Fixture modeled on real `lsblk -J -o NAME,PKNAME,TYPE` /
// `lsblk -J -b -o NAME,PATH,SIZE,FSTYPE,LABEL,UUID,MOUNTPOINT,RM,TYPE,PKNAME`
// output for a typical small VPS: one system disk (sda, partitioned, ext4
// root) plus one plugged-in USB drive (sdb, single ext4 partition) and one
// unpartitioned USB stick with a filesystem directly on the whole disk
// (sdc, vfat).
const fixtureLsblk = `{
  "blockdevices": [
    {"name":"sda","path":"/dev/sda","size":"500000000000","fstype":null,"label":null,"uuid":null,"mountpoint":null,"rm":"0","type":"disk","pkname":null,
     "children":[
        {"name":"sda1","path":"/dev/sda1","size":"1000000000","fstype":"vfat","label":"ESP","uuid":"AAAA-1111","mountpoint":"/boot/efi","rm":"0","type":"part","pkname":"sda"},
        {"name":"sda2","path":"/dev/sda2","size":"499000000000","fstype":"ext4","label":null,"uuid":"root-uuid-1234","mountpoint":"/","rm":"0","type":"part","pkname":"sda"}
     ]},
    {"name":"sdb","path":"/dev/sdb","size":"64000000000","fstype":null,"label":null,"uuid":null,"mountpoint":null,"rm":"1","type":"disk","pkname":null,
     "children":[
        {"name":"sdb1","path":"/dev/sdb1","size":"64000000000","fstype":"ext4","label":"BACKUP","uuid":"drive-uuid-5678","mountpoint":null,"rm":"1","type":"part","pkname":"sdb"}
     ]},
    {"name":"sdc","path":"/dev/sdc","size":"16000000000","fstype":"vfat","label":"STICK","uuid":"drive-uuid-9999","mountpoint":null,"rm":"1","type":"disk","pkname":null}
  ]
}`

func mustParseFixture(t *testing.T) lsblkOutput {
	t.Helper()
	var out lsblkOutput
	if err := json.Unmarshal([]byte(fixtureLsblk), &out); err != nil {
		t.Fatalf("parse fixture: %v", err)
	}
	return out
}

func TestResolveRootDiskName(t *testing.T) {
	parsed := mustParseFixture(t)
	// The root filesystem's source device is sda2 (a partition); the
	// PKNAME chain must resolve up to the top-level disk, sda.
	got := resolveRootDiskName(parsed, "sda2")
	if got != "sda" {
		t.Fatalf("resolveRootDiskName(sda2) = %q, want sda", got)
	}
}

func TestResolveRootDiskName_AlreadyTopLevel(t *testing.T) {
	parsed := mustParseFixture(t)
	got := resolveRootDiskName(parsed, "sda")
	if got != "sda" {
		t.Fatalf("resolveRootDiskName(sda) = %q, want sda", got)
	}
}

// TestFilterDrives_NeverOffersSystemDisk is the load-bearing safety test
// in this package: whatever else changes, the system disk and every one
// of its partitions must never appear in the list a client can mount or
// unmount, even though it's a real, mountable-looking partition (ext4,
// non-empty FSType) that would otherwise pass every other check.
func TestFilterDrives_NeverOffersSystemDisk(t *testing.T) {
	parsed := mustParseFixture(t)
	drives := filterDrives(parsed, "sda")

	for _, d := range drives {
		if d.UUID == "root-uuid-1234" {
			t.Fatalf("filterDrives offered the root filesystem's own partition: %+v", d)
		}
		if d.Device == "/dev/sda1" || d.Device == "/dev/sda2" {
			t.Fatalf("filterDrives offered a partition of the system disk: %+v", d)
		}
	}
}

func TestFilterDrives_OffersRemovableDrives(t *testing.T) {
	parsed := mustParseFixture(t)
	drives := filterDrives(parsed, "sda")

	byUUID := map[string]Drive{}
	for _, d := range drives {
		byUUID[d.UUID] = d
	}

	sdb1, ok := byUUID["drive-uuid-5678"]
	if !ok {
		t.Fatalf("expected sdb1 (a real partition on a non-system disk) to be offered; got %+v", drives)
	}
	if sdb1.Mounted {
		t.Errorf("sdb1 should be reported unmounted, got Mounted=true")
	}
	if sdb1.Filesystem != "ext4" || !sdb1.Removable {
		t.Errorf("sdb1 fields wrong: %+v", sdb1)
	}

	sdc, ok := byUUID["drive-uuid-9999"]
	if !ok {
		t.Fatalf("expected sdc (unpartitioned disk with a filesystem directly on it) to be offered; got %+v", drives)
	}
	if sdc.Device != "/dev/sdc" {
		t.Errorf("sdc device wrong: %+v", sdc)
	}
}

func TestFilterDrives_SkipsEmptyFilesystem(t *testing.T) {
	parsed := mustParseFixture(t)
	drives := filterDrives(parsed, "sda")
	for _, d := range drives {
		if d.Device == "/dev/sdb" {
			t.Errorf("the whole disk /dev/sdb has no filesystem of its own (its partition sdb1 does) and should not be offered: %+v", d)
		}
	}
}

func TestMountArgs(t *testing.T) {
	cases := []struct {
		fstype  string
		wantErr bool
	}{
		{"ext4", false},
		{"vfat", false},
		{"exfat", false},
		{"ntfs", false},
		{"", true},
	}
	for _, c := range cases {
		args, err := mountArgs(c.fstype, "/dev/sdb1", "/var/lib/mangrove-drives/x")
		if c.wantErr {
			if err == nil {
				t.Errorf("mountArgs(%q): expected error, got none", c.fstype)
			}
			continue
		}
		if err != nil {
			t.Errorf("mountArgs(%q): unexpected error: %v", c.fstype, err)
			continue
		}
		if len(args) == 0 {
			t.Errorf("mountArgs(%q): no candidate argv sets returned", c.fstype)
		}
		for _, a := range args {
			found := false
			for _, tok := range a {
				if tok == "nosuid,nodev" {
					found = true
				}
			}
			if !found {
				t.Errorf("mountArgs(%q): argv %v missing nosuid,nodev", c.fstype, a)
			}
		}
	}
	if c := "ntfs"; true {
		args, _ := mountArgs(c, "/dev/sdb1", "/x")
		if len(args) < 2 {
			t.Errorf("mountArgs(ntfs): expected an ntfs3 + ntfs-3g fallback pair, got %v", args)
		}
	}
}
