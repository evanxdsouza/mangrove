package orchestrator

import (
	"context"
	"strings"
	"testing"

	"github.com/evanxdsouza/mangrove/internal/mountd"
)

// fakeMountd satisfies MountdClient without a running mangrove-mountd.
// Drives are seeded directly; Mount flips Mounted/MountPath the same way
// the real helper would.
type fakeMountd struct {
	drives         []mountd.Drive
	unmountedUUIDs []string
}

func (f *fakeMountd) List(ctx context.Context) ([]mountd.Drive, error) {
	out := make([]mountd.Drive, len(f.drives))
	copy(out, f.drives)
	return out, nil
}

func (f *fakeMountd) Mount(ctx context.Context, uuid string) (mountd.Drive, error) {
	for i := range f.drives {
		if f.drives[i].UUID == uuid {
			f.drives[i].Mounted = true
			f.drives[i].MountPath = "/var/lib/mangrove-drives/" + uuid
			return f.drives[i], nil
		}
	}
	return mountd.Drive{}, errNotFound("no such drive")
}

func (f *fakeMountd) Unmount(ctx context.Context, uuid string) error {
	f.unmountedUUIDs = append(f.unmountedUUIDs, uuid)
	for i := range f.drives {
		if f.drives[i].UUID == uuid {
			f.drives[i].Mounted = false
			f.drives[i].MountPath = ""
		}
	}
	return nil
}

type errNotFound string

func (e errNotFound) Error() string { return string(e) }

func newStorageTestOrchestrator(t *testing.T, fm *fakeMountd) (*Orchestrator, *fakeTemplateExecutor) {
	t.Helper()
	o, _, _ := newTestOrchestrator(t)
	fe := &fakeTemplateExecutor{}
	o.Exec = fe
	o.Mountd = fm
	return o, fe
}

func TestCreateNASShare_RequiresMountedDrive(t *testing.T) {
	fm := &fakeMountd{drives: []mountd.Drive{{UUID: "u1", Device: "/dev/sdb1", Filesystem: "ext4"}}}
	o, _ := newStorageTestOrchestrator(t, fm)

	_, err := o.CreateNASShare(context.Background(), CreateNASShareParams{
		DriveUUID: "u1", Slug: "backup", ShareName: "backup", Username: "user", Password: "pass1234",
	})
	if err == nil || !strings.Contains(err.Error(), "not mounted") {
		t.Fatalf("expected a 'not mounted' error, got %v", err)
	}
}

func TestCreateNASShare_Succeeds(t *testing.T) {
	fm := &fakeMountd{drives: []mountd.Drive{{UUID: "u1", Device: "/dev/sdb1", Filesystem: "ext4", Mounted: true, MountPath: "/var/lib/mangrove-drives/u1"}}}
	o, fe := newStorageTestOrchestrator(t, fm)

	dep, err := o.CreateNASShare(context.Background(), CreateNASShareParams{
		DriveUUID: "u1", Slug: "backup", ShareName: "backup", Username: "user", Password: "pass1234",
	})
	if err != nil {
		t.Fatalf("CreateNASShare: %v", err)
	}
	if dep.Status != "running" {
		t.Errorf("expected deployment status running, got %q", dep.Status)
	}
	if dep.BuildStrategy != "image" || dep.ImageRef != nasImageRef {
		t.Errorf("unexpected deployment build config: %+v", dep)
	}

	if len(fe.runSpecs) != 1 {
		t.Fatalf("expected exactly one Run() call, got %d", len(fe.runSpecs))
	}
	rs := fe.runSpecs[0]
	if rs.HostMount == nil || rs.HostMount.HostPath != "/var/lib/mangrove-drives/u1" || rs.HostMount.ContainerPath != "/share" {
		t.Errorf("unexpected HostMount: %+v", rs.HostMount)
	}
	if rs.HostPort == nil || *rs.HostPort != smbPort {
		t.Errorf("expected HostPort %d, got %v", smbPort, rs.HostPort)
	}
	if !rs.PublicBind {
		t.Error("expected PublicBind=true so the share is reachable from the LAN, not just 127.0.0.1")
	}
	if rs.HostMount.ReadOnly {
		t.Error("a NAS share must be writable, not read-only")
	}

	services, err := o.Store.ListServices(context.Background(), dep.ID)
	if err != nil || len(services) != 1 {
		t.Fatalf("ListServices: %v (%d services)", err, len(services))
	}
	svc := services[0]
	if svc.DirectPublishPort == nil || *svc.DirectPublishPort != smbPort {
		t.Errorf("expected DirectPublishPort %d recorded on the service, got %v", smbPort, svc.DirectPublishPort)
	}
	if svc.HostMountSource != "/var/lib/mangrove-drives/u1" || svc.HostMountTarget != "/share" {
		t.Errorf("unexpected host mount fields: %+v", svc)
	}
}

func TestCreateNASShare_RefusesDoubleShare(t *testing.T) {
	fm := &fakeMountd{drives: []mountd.Drive{{UUID: "u1", Device: "/dev/sdb1", Filesystem: "ext4", Mounted: true, MountPath: "/var/lib/mangrove-drives/u1"}}}
	o, _ := newStorageTestOrchestrator(t, fm)

	if _, err := o.CreateNASShare(context.Background(), CreateNASShareParams{
		DriveUUID: "u1", Slug: "backup", ShareName: "backup", Username: "user", Password: "pass1234",
	}); err != nil {
		t.Fatalf("first CreateNASShare: %v", err)
	}

	_, err := o.CreateNASShare(context.Background(), CreateNASShareParams{
		DriveUUID: "u1", Slug: "backup2", ShareName: "backup2", Username: "user", Password: "pass1234",
	})
	if err == nil || !strings.Contains(err.Error(), "already shared") {
		t.Fatalf("expected an 'already shared' error, got %v", err)
	}
}

// TestDeploy_RefusesNASShare guards the guard: a NAS share's service can
// never be redeployed/rolled back/scaled through the normal blue/green
// Deploy() path (see deploy.go's DirectPublishPort check), since that path
// can't hold two containers on the same exclusive host port during its
// health-check gate.
func TestDeploy_RefusesNASShare(t *testing.T) {
	fm := &fakeMountd{drives: []mountd.Drive{{UUID: "u1", Device: "/dev/sdb1", Filesystem: "ext4", Mounted: true, MountPath: "/var/lib/mangrove-drives/u1"}}}
	o, _ := newStorageTestOrchestrator(t, fm)

	dep, err := o.CreateNASShare(context.Background(), CreateNASShareParams{
		DriveUUID: "u1", Slug: "backup", ShareName: "backup", Username: "user", Password: "pass1234",
	})
	if err != nil {
		t.Fatalf("CreateNASShare: %v", err)
	}

	_, err = o.Deploy(context.Background(), DeployRequest{DeploymentID: dep.ID, TriggeredBy: "api"})
	if err == nil || !strings.Contains(err.Error(), "doesn't support redeploy") {
		t.Fatalf("expected Deploy() to refuse a NAS share, got %v", err)
	}
}

func TestUnmountDrive_RefusesWhileShared(t *testing.T) {
	fm := &fakeMountd{drives: []mountd.Drive{{UUID: "u1", Device: "/dev/sdb1", Filesystem: "ext4", Mounted: true, MountPath: "/var/lib/mangrove-drives/u1"}}}
	o, _ := newStorageTestOrchestrator(t, fm)

	if _, err := o.CreateNASShare(context.Background(), CreateNASShareParams{
		DriveUUID: "u1", Slug: "backup", ShareName: "backup", Username: "user", Password: "pass1234",
	}); err != nil {
		t.Fatalf("CreateNASShare: %v", err)
	}

	err := o.UnmountDrive(context.Background(), "u1")
	if err == nil || !strings.Contains(err.Error(), "shared as deployment") {
		t.Fatalf("expected UnmountDrive to refuse a shared drive, got %v", err)
	}
	if len(fm.unmountedUUIDs) != 0 {
		t.Error("Unmount should never have reached the mountd client")
	}
}

func TestListDrives_NilMountd(t *testing.T) {
	o, _, _ := newTestOrchestrator(t)
	o.Exec = &fakeTemplateExecutor{}
	// o.Mountd left nil -- a box with no storage helper installed.
	_, err := o.ListDrives(context.Background())
	if err != mountd.ErrUnavailable {
		t.Fatalf("ListDrives with nil Mountd = %v, want mountd.ErrUnavailable", err)
	}
}
