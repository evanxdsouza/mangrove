// Package mountd defines the protocol between Mangrove's main control-plane
// process (unprivileged, sandboxed by mangrove.service -- see
// deploy/systemd/mangrove.service) and mangrove-mountd, a small separate
// privileged helper that owns mounting/unmounting removable drives. The
// main process never gains mount capabilities itself; it only ever talks
// to the helper over a local Unix domain socket. See docs/storage.md for
// the full design rationale.
package mountd

// Drive is one removable block device or partition mangrove-mountd is
// willing to consider -- never the disk backing the root filesystem or
// anything already mounted at a system path (see server.go's listDrives).
type Drive struct {
	// UUID is the filesystem UUID (from blkid/lsblk), used as the stable
	// identifier across replug/reboot instead of the kernel-assigned
	// device path (/dev/sdb1 today can be /dev/sdc1 tomorrow). A drive
	// with no filesystem UUID (rare, but possible for some exFAT/vfat
	// tools) is reported but can't be mounted through this protocol.
	UUID       string `json:"uuid"`
	Device     string `json:"device"` // e.g. /dev/sdb1
	Label      string `json:"label,omitempty"`
	Filesystem string `json:"filesystem,omitempty"`
	SizeBytes  int64  `json:"size_bytes"`
	Removable  bool   `json:"removable"`
	Mounted    bool   `json:"mounted"`
	// MountPath is set when Mounted is true -- the host path Mangrove's
	// main process can bind-mount into a container. Only meaningful when
	// it falls under the helper's configured mount root; a drive the OS
	// auto-mounted somewhere else is reported as mounted but with a path
	// outside mangrove-mountd's control (Mount/Unmount will refuse it).
	MountPath string `json:"mount_path,omitempty"`
}

// Action names understood by the helper's single-request-per-connection
// protocol (see server.go). Deliberately a closed, tiny set -- this socket
// speaks exactly three verbs, never arbitrary commands.
const (
	ActionList    = "list"
	ActionMount   = "mount"
	ActionUnmount = "unmount"
)

// Request is one JSON object, newline-terminated, sent over a freshly
// dialed connection to the helper's socket. The connection is closed after
// one Request/Response round trip -- there's no persistent session.
type Request struct {
	Action string `json:"action"`
	UUID   string `json:"uuid,omitempty"` // required for mount/unmount
}

// Response is the helper's single JSON reply, newline-terminated.
type Response struct {
	OK     bool    `json:"ok"`
	Error  string  `json:"error,omitempty"`
	Drives []Drive `json:"drives,omitempty"` // ActionList
	Drive  *Drive  `json:"drive,omitempty"`  // ActionMount
}
