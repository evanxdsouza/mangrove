// Storage/NAS shares: turn a plugged-in drive into an SMB share reachable
// from other devices on the LAN. See docs/storage.md for the full design
// (why a separate privileged helper does the mounting, why this doesn't go
// through the normal blue/green Deploy() path, why SMB needs a direct host
// port publish instead of a Caddy route).
package orchestrator

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/evanxdsouza/mangrove/internal/executor"
	"github.com/evanxdsouza/mangrove/internal/models"
	"github.com/evanxdsouza/mangrove/internal/mountd"
	"github.com/evanxdsouza/mangrove/internal/store"
)

// smbPort is the standard SMB port. Published directly on the host (see
// executor.RunSpec.PublicBind) rather than allocated from port_registry --
// SMB clients expect the well-known port, and Caddy can't proxy it anyway.
const smbPort = 445

const nasImageRef = "dperson/samba"

// storageProjectName/Slug: every NAS share lands in one shared project
// (auto-created on first use, same convention simple mode uses for its
// one-click templates) rather than fragmenting into a project per drive.
const storageProjectName = "Storage"
const storageProjectSlug = "storage"

// MountdClient is the subset of *mountd.Client the orchestrator depends
// on -- an interface so tests can fake it without a running mangrove-mountd.
// A nil Mountd on the Orchestrator is valid (mirrors Proxy/Notifier/GHStatus):
// every method below reports mountd.ErrUnavailable rather than panicking, so
// a box with no storage helper installed just has an empty Storage page.
type MountdClient interface {
	List(ctx context.Context) ([]mountd.Drive, error)
	Mount(ctx context.Context, uuid string) (mountd.Drive, error)
	Unmount(ctx context.Context, uuid string) error
}

// ListDrives returns every removable drive mangrove-mountd is willing to
// consider, mounted or not.
func (o *Orchestrator) ListDrives(ctx context.Context) ([]mountd.Drive, error) {
	if o.Mountd == nil {
		return nil, mountd.ErrUnavailable
	}
	return o.Mountd.List(ctx)
}

// MountDrive mounts a drive under mangrove-mountd's controlled root. Must
// succeed before CreateNASShare can share it.
func (o *Orchestrator) MountDrive(ctx context.Context, uuid string) (mountd.Drive, error) {
	if o.Mountd == nil {
		return mountd.Drive{}, mountd.ErrUnavailable
	}
	return o.Mountd.Mount(ctx, uuid)
}

// UnmountDrive unmounts a drive, refusing if a live NAS share is currently
// bind-mounting it -- unmounting out from under a running container's bind
// mount can corrupt writes in flight, and would leave the share's
// container silently serving a now-empty directory.
func (o *Orchestrator) UnmountDrive(ctx context.Context, uuid string) error {
	if o.Mountd == nil {
		return mountd.ErrUnavailable
	}
	inUse, err := o.driveInUseBy(ctx, uuid)
	if err != nil {
		return fmt.Errorf("check active shares: %w", err)
	}
	if inUse != nil {
		return fmt.Errorf("drive is shared as deployment %d (%q) -- delete that share first", inUse.DeploymentID, inUse.Name)
	}
	return o.Mountd.Unmount(ctx, uuid)
}

// driveInUseBy returns the NAS-share service currently bind-mounting the
// drive with this UUID, or nil if none. Matches on mount path rather than
// UUID directly since that's what's actually recorded on the service row
// (HostMountSource) -- mangrove-mountd always mounts under
// <MountRoot>/<uuid>, so the UUID is the path's final element. Compared via
// filepath.Base rather than a raw string-suffix check, which would
// false-positive whenever one UUID happens to be a character-for-character
// suffix of another (e.g. "1234" matching a path ending in "...a1234").
func (o *Orchestrator) driveInUseBy(ctx context.Context, uuid string) (*models.Service, error) {
	if uuid == "" {
		return nil, nil
	}
	shares, err := o.Store.ListNASShares(ctx)
	if err != nil {
		return nil, err
	}
	for i := range shares {
		if filepath.Base(shares[i].HostMountSource) == uuid {
			return &shares[i], nil
		}
	}
	return nil, nil
}

// CreateNASShareParams describes a new SMB share over an already-mounted
// drive.
type CreateNASShareParams struct {
	DriveUUID string
	// Slug names the deployment (must be unique within the Storage
	// project, same rule as any deployment slug); ShareName is what shows
	// up as the SMB share name itself (smb://host/ShareName).
	Slug      string
	ShareName string
	Username  string
	Password  string
}

// CreateNASShare mounts (if not already) and shares a drive over SMB.
// Deliberately does NOT go through Deploy(): a NAS share holds an
// exclusive host port (445) for its entire lifetime, which the blue/green
// swap can't accommodate (old and new containers briefly coexist on the
// same host port during a normal deploy's health-check gate -- see
// deploy.go's comment on why RunSpec.HostPort is otherwise never set).
// A NAS share is effectively immutable once created: to change its
// config, delete and recreate rather than redeploy (Deploy() itself
// refuses a redeploy/rollback/scale of a service with DirectPublishPort
// set -- see its guard).
func (o *Orchestrator) CreateNASShare(ctx context.Context, p CreateNASShareParams) (models.Deployment, error) {
	if o.Mountd == nil {
		return models.Deployment{}, mountd.ErrUnavailable
	}
	if p.Slug == "" || p.ShareName == "" || p.Username == "" || p.Password == "" {
		return models.Deployment{}, fmt.Errorf("slug, share name, username, and password are all required")
	}

	drives, err := o.Mountd.List(ctx)
	if err != nil {
		return models.Deployment{}, fmt.Errorf("list drives: %w", err)
	}
	var drive *mountd.Drive
	for i := range drives {
		if drives[i].UUID == p.DriveUUID {
			drive = &drives[i]
			break
		}
	}
	if drive == nil {
		return models.Deployment{}, fmt.Errorf("no such drive")
	}
	if !drive.Mounted {
		return models.Deployment{}, fmt.Errorf("drive is not mounted -- mount it first")
	}

	if existing, err := o.driveInUseBy(ctx, p.DriveUUID); err != nil {
		return models.Deployment{}, fmt.Errorf("check existing shares: %w", err)
	} else if existing != nil {
		return models.Deployment{}, fmt.Errorf("this drive is already shared (deployment %d)", existing.DeploymentID)
	}

	projectID, err := o.storageProjectID(ctx)
	if err != nil {
		return models.Deployment{}, fmt.Errorf("find or create Storage project: %w", err)
	}

	dep, err := o.Store.CreateDeployment(ctx, store.CreateDeploymentParams{
		ProjectID:     projectID,
		Name:          p.ShareName,
		Slug:          p.Slug,
		BuildStrategy: "image",
		ImageRef:      nasImageRef,
		RootPath:      ".",
	})
	if err != nil {
		return models.Deployment{}, fmt.Errorf("create deployment: %w", err)
	}

	const serviceName = "samba"
	containerName := fmt.Sprintf("mangrove-%s-%s", p.Slug, serviceName)
	directPort := smbPort
	svc, err := o.Store.CreateService(ctx, store.CreateServiceParams{
		DeploymentID:      dep.ID,
		Name:              serviceName,
		ContainerName:     containerName,
		InternalPort:      smbPort,
		IsInternalOnly:    true, // no Caddy route -- reachability is the direct host-port publish below
		CPULimitCores:     1,
		MemoryLimitMB:     256,
		RestartPolicy:     "unless-stopped",
		Command:           sambaCommand(p.ShareName, p.Username, p.Password),
		DirectPublishPort: &directPort,
		HostMountSource:   drive.MountPath,
		HostMountTarget:   "/share",
	})
	if err != nil {
		return models.Deployment{}, fmt.Errorf("create service: %w", err)
	}

	runSpec := executor.RunSpec{
		ImageRef:      nasImageRef,
		ContainerName: containerName,
		Network:       o.Config.NetworkName,
		NetworkAlias:  containerName,
		InternalPort:  smbPort,
		HostPort:      &directPort,
		PublicBind:    true,
		CPULimitCores: svc.CPULimitCores,
		MemoryLimitMB: svc.MemoryLimitMB,
		RestartPolicy: svc.RestartPolicy,
		Command:       svc.Command,
		CgroupParent:  o.Config.CgroupParent,
		HostMount:     &executor.HostMount{HostPath: drive.MountPath, ContainerPath: "/share", ReadOnly: false},
	}

	result, err := o.Exec.Run(ctx, runSpec)
	if err != nil {
		o.Store.UpdateServiceStatus(ctx, svc.ID, "failed")
		o.Store.UpdateDeploymentStatus(ctx, dep.ID, "failed")
		return models.Deployment{}, fmt.Errorf("run samba container: %w", err)
	}

	// No HTTP health check exists for SMB -- same "still running after a
	// grace period" fallback Deploy()'s waitHealthy uses for a
	// health-check-less service like Postgres.
	time.Sleep(3 * time.Second)
	if _, err := o.Exec.Stats(ctx, containerName); err != nil {
		o.Exec.Stop(ctx, result.ContainerID, 10*time.Second)
		o.Exec.Remove(ctx, result.ContainerID)
		o.Store.UpdateServiceStatus(ctx, svc.ID, "failed")
		o.Store.UpdateDeploymentStatus(ctx, dep.ID, "failed")
		return models.Deployment{}, fmt.Errorf("samba container did not stay running")
	}

	if err := o.Store.UpdateServiceRuntime(ctx, svc.ID, nasImageRef, result.ContainerID, "running"); err != nil {
		o.Log.Warn("create NAS share: failed to record service runtime", "service_id", svc.ID, "error", err)
	}
	if err := o.Store.UpdateDeploymentStatus(ctx, dep.ID, "running"); err != nil {
		o.Log.Warn("create NAS share: failed to update deployment status", "deployment_id", dep.ID, "error", err)
	}
	o.Store.TouchDeploymentDeployed(ctx, dep.ID)

	return o.Store.GetDeployment(ctx, dep.ID)
}

// storageProjectID finds or creates the one shared "Storage" project every
// NAS share lands in, in the default workspace.
func (o *Orchestrator) storageProjectID(ctx context.Context) (int64, error) {
	projects, err := o.Store.ListProjectsByWorkspace(ctx, 1)
	if err != nil {
		return 0, err
	}
	for _, p := range projects {
		if p.Slug == storageProjectSlug {
			return p.ID, nil
		}
	}
	p, err := o.Store.CreateProject(ctx, 1, storageProjectName, storageProjectSlug, "Storage/NAS shares created from the Storage page.")
	if err != nil {
		return 0, err
	}
	return p.ID, nil
}

// sambaCommand builds the dperson/samba image's argv (see
// https://github.com/dperson/samba): -u registers one user, -s defines one
// share. Passed as RunSpec.Command / CreateServiceParams.Command, which
// docker.go runs as exec-form args after the image ref -- never through a
// shell, so a share name or password containing a `;` or shell
// metacharacter can't break out of the image's own arg parsing into
// something docker/the shell reinterprets.
func sambaCommand(shareName, username, password string) []string {
	return []string{
		"-u", fmt.Sprintf("%s;%s", username, password),
		"-s", fmt.Sprintf("%s;/share;yes;no;no;%s;none;none;Mangrove-managed share", shareName, username),
	}
}
