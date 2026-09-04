package mountd

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Server is mangrove-mountd itself: the privileged half of the mountd
// protocol. It never runs inside the main mangrove process -- only
// cmd/mangrove-mountd constructs one. Everything it does is deliberately
// narrow: enumerate removable block devices, mount one under MountRoot,
// unmount one it previously mounted. No shell strings are ever built from
// caller input -- every external command runs with an explicit argv, and
// every device path it acts on is one *this* process just found via lsblk,
// never a raw string a client supplies.
type Server struct {
	SocketPath string
	// MountRoot is the directory every drive gets mounted under, one
	// subdirectory per filesystem UUID (e.g. /var/lib/mangrove-drives/<uuid>).
	// Unmount refuses to touch anything outside this prefix.
	MountRoot string
	// SocketGroup, if set, is a group name the socket file is chgrp'd to
	// after creation (mode 0660) so the unprivileged mangrove user can
	// reach it without the socket being world-accessible. Typically
	// "mangrove-mount" -- see deploy/systemd/mangrove-mountd.service.
	SocketGroup string
	Log         *slog.Logger
}

// ListenAndServe creates (replacing any stale socket file) the Unix socket
// and serves requests until ctx is cancelled. One goroutine per connection;
// each connection handles exactly one request/response round trip.
func (s *Server) ListenAndServe(ctx context.Context) error {
	if err := os.MkdirAll(s.MountRoot, 0755); err != nil {
		return fmt.Errorf("create mount root: %w", err)
	}
	_ = os.Remove(s.SocketPath) // stale socket from a prior crashed run
	ln, err := net.Listen("unix", s.SocketPath)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", s.SocketPath, err)
	}
	defer ln.Close()

	if err := os.Chmod(s.SocketPath, 0660); err != nil {
		s.Log.Warn("chmod socket failed", "error", err)
	}
	if s.SocketGroup != "" {
		if g, err := user.LookupGroup(s.SocketGroup); err == nil {
			gid, _ := strconv.Atoi(g.Gid)
			if err := os.Chown(s.SocketPath, -1, gid); err != nil {
				s.Log.Warn("chown socket failed", "group", s.SocketGroup, "error", err)
			}
		} else {
			s.Log.Warn("socket group not found -- socket stays root-only", "group", s.SocketGroup, "error", err)
		}
	}

	go func() {
		<-ctx.Done()
		ln.Close()
	}()

	for {
		conn, err := ln.Accept()
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return fmt.Errorf("accept: %w", err)
		}
		go s.handle(ctx, conn)
	}
}

func (s *Server) handle(ctx context.Context, conn net.Conn) {
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(30 * time.Second))

	var req Request
	if err := json.NewDecoder(bufio.NewReader(conn)).Decode(&req); err != nil {
		s.reply(conn, Response{OK: false, Error: "invalid request: " + err.Error()})
		return
	}

	switch req.Action {
	case ActionList:
		drives, err := s.listDrives(ctx)
		if err != nil {
			s.reply(conn, Response{OK: false, Error: err.Error()})
			return
		}
		s.reply(conn, Response{OK: true, Drives: drives})
	case ActionMount:
		if req.UUID == "" {
			s.reply(conn, Response{OK: false, Error: "uuid is required"})
			return
		}
		d, err := s.mount(ctx, req.UUID)
		if err != nil {
			s.reply(conn, Response{OK: false, Error: err.Error()})
			return
		}
		s.reply(conn, Response{OK: true, Drive: &d})
	case ActionUnmount:
		if req.UUID == "" {
			s.reply(conn, Response{OK: false, Error: "uuid is required"})
			return
		}
		if err := s.unmount(ctx, req.UUID); err != nil {
			s.reply(conn, Response{OK: false, Error: err.Error()})
			return
		}
		s.reply(conn, Response{OK: true})
	default:
		s.reply(conn, Response{OK: false, Error: "unknown action " + req.Action})
	}
}

func (s *Server) reply(conn net.Conn, resp Response) {
	if err := json.NewEncoder(conn).Encode(resp); err != nil {
		s.Log.Warn("write response failed", "error", err)
	}
}

// ---- lsblk-backed device enumeration ----

// lsblkDevice mirrors the fields we ask lsblk for. util-linux versions
// disagree on whether numeric/boolean fields are quoted in JSON output, so
// every field that varies uses a flexible unmarshaler below rather than a
// fixed Go type.
type lsblkDevice struct {
	Name       string        `json:"name"`
	Path       string        `json:"path"`
	Size       flexInt64     `json:"size"`
	FSType     string        `json:"fstype"`
	Label      string        `json:"label"`
	UUID       string        `json:"uuid"`
	MountPoint string        `json:"mountpoint"`
	RM         flexBool      `json:"rm"`
	Type       string        `json:"type"`
	PKName     string        `json:"pkname"`
	Children   []lsblkDevice `json:"children"`
}

type lsblkOutput struct {
	BlockDevices []lsblkDevice `json:"blockdevices"`
}

// flexInt64/flexBool unmarshal a JSON field that different lsblk versions
// emit as either a native number/bool or a quoted string.
type flexInt64 int64

func (f *flexInt64) UnmarshalJSON(b []byte) error {
	s := strings.Trim(string(b), `"`)
	if s == "" || s == "null" {
		*f = 0
		return nil
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return nil // tolerate an unparseable size rather than failing enumeration entirely
	}
	*f = flexInt64(n)
	return nil
}

type flexBool bool

func (f *flexBool) UnmarshalJSON(b []byte) error {
	s := strings.Trim(string(b), `"`)
	*f = s == "1" || s == "true"
	return nil
}

// rootDiskNames returns the kernel device name (e.g. "sda", "nvme0n1") of
// the disk backing "/", plus every one of its own descendant partition
// names -- the set of devices listDrives must never offer, mount, or
// unmount, no matter what a caller asks for. Resolved fresh on every list
// call rather than cached, since it's cheap and this is exactly the check
// we can't afford to get stale.
func rootDiskName(ctx context.Context) (string, error) {
	out, err := exec.CommandContext(ctx, "findmnt", "-no", "SOURCE", "/").CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("findmnt /: %w: %s", err, strings.TrimSpace(string(out)))
	}
	rootSource := strings.TrimSpace(string(out)) // e.g. /dev/sda1, or /dev/mapper/... for LVM
	rootDev := filepath.Base(rootSource)

	// Walk lsblk to find rootDev and follow PKNAME up to the top-level disk.
	lsOut, err := exec.CommandContext(ctx, "lsblk", "-J", "-o", "NAME,PKNAME,TYPE").CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("lsblk: %w: %s", err, strings.TrimSpace(string(lsOut)))
	}
	var parsed lsblkOutput
	if err := json.Unmarshal(lsOut, &parsed); err != nil {
		return "", fmt.Errorf("parse lsblk output: %w", err)
	}
	return resolveRootDiskName(parsed, rootDev), nil
}

// resolveRootDiskName walks lsblk's PKNAME chain from rootDev up to its
// top-level disk. Split out from rootDiskName so the walk itself (the part
// that actually decides what's "the system disk") can be unit tested
// against fixture lsblk output without shelling out.
func resolveRootDiskName(parsed lsblkOutput, rootDev string) string {
	byName := map[string]lsblkDevice{}
	var flatten func(devs []lsblkDevice)
	flatten = func(devs []lsblkDevice) {
		for _, d := range devs {
			byName[d.Name] = d
			flatten(d.Children)
		}
	}
	flatten(parsed.BlockDevices)

	name := rootDev
	for i := 0; i < 8; i++ { // bounded walk up the parent chain -- LVM/dm-crypt can stack several layers
		d, ok := byName[name]
		if !ok || d.PKName == "" {
			return name // reached a top-level disk (or an unrecognized device -- treat it as the boundary)
		}
		name = d.PKName
	}
	return name
}

// listDrives enumerates every partition (or unpartitioned whole disk with a
// filesystem directly on it) that is NOT the root disk or one of its
// partitions, and reports whether/where it's currently mounted.
func (s *Server) listDrives(ctx context.Context) ([]Drive, error) {
	rootDisk, err := rootDiskName(ctx)
	if err != nil {
		return nil, err
	}

	out, err := exec.CommandContext(ctx, "lsblk", "-J", "-b",
		"-o", "NAME,PATH,SIZE,FSTYPE,LABEL,UUID,MOUNTPOINT,RM,TYPE,PKNAME").CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("lsblk: %w: %s", err, strings.TrimSpace(string(out)))
	}
	var parsed lsblkOutput
	if err := json.Unmarshal(out, &parsed); err != nil {
		return nil, fmt.Errorf("parse lsblk output: %w", err)
	}
	return filterDrives(parsed, rootDisk), nil
}

// filterDrives turns parsed lsblk output into the Drive list listDrives
// reports: every mountable partition (or unpartitioned whole disk with a
// filesystem directly on it) except the root disk and anything under it.
// Pure and side-effect-free so the actual safety boundary -- what counts
// as "the system disk" -- is unit-testable against fixture data rather
// than only ever exercised on real hardware.
func filterDrives(parsed lsblkOutput, rootDisk string) []Drive {
	var drives []Drive
	var walk func(devs []lsblkDevice, topLevelName string)
	walk = func(devs []lsblkDevice, topLevelName string) {
		for _, d := range devs {
			top := topLevelName
			if top == "" {
				top = d.Name
			}
			if top == rootDisk {
				continue // this device or one of its partitions backs "/" -- never touch it
			}
			if (d.Type == "part" || d.Type == "disk") && d.FSType != "" {
				drives = append(drives, Drive{
					UUID:       d.UUID,
					Device:     d.Path,
					Label:      d.Label,
					Filesystem: d.FSType,
					SizeBytes:  int64(d.Size),
					Removable:  bool(d.RM),
					Mounted:    d.MountPoint != "",
					MountPath:  d.MountPoint,
				})
			}
			walk(d.Children, top)
		}
	}
	walk(parsed.BlockDevices, "")
	return drives
}

// findByUUID re-lists and returns the single drive matching uuid, or an
// error -- used by mount/unmount so every mutating call re-validates
// against a fresh device snapshot rather than trusting a caller-supplied
// device path (which could be stale or, from an untrusted caller, simply
// false).
func (s *Server) findByUUID(ctx context.Context, uuid string) (Drive, error) {
	drives, err := s.listDrives(ctx)
	if err != nil {
		return Drive{}, err
	}
	for _, d := range drives {
		if d.UUID == uuid {
			return d, nil
		}
	}
	return Drive{}, fmt.Errorf("no such drive (uuid %q not found, or it backs the root filesystem)", uuid)
}

func (s *Server) mount(ctx context.Context, uuid string) (Drive, error) {
	d, err := s.findByUUID(ctx, uuid)
	if err != nil {
		return Drive{}, err
	}
	if d.Mounted {
		return Drive{}, fmt.Errorf("already mounted at %s", d.MountPath)
	}
	if d.Filesystem == "" {
		return Drive{}, fmt.Errorf("no filesystem detected on %s", d.Device)
	}

	// uuid is validated above against lsblk's own output (a device this
	// process just enumerated), so it's safe to use directly in a path --
	// but filepath.Join + a prefix check below is cheap insurance against
	// a pathological UUID string regardless.
	target := filepath.Join(s.MountRoot, uuid)
	if !strings.HasPrefix(target, filepath.Clean(s.MountRoot)+string(filepath.Separator)) {
		return Drive{}, fmt.Errorf("refusing to mount outside mount root")
	}
	if err := os.MkdirAll(target, 0755); err != nil {
		return Drive{}, fmt.Errorf("create mount point: %w", err)
	}

	args, err := mountArgs(d.Filesystem, d.Device, target)
	if err != nil {
		_ = os.Remove(target)
		return Drive{}, err
	}
	var lastErr error
	for _, a := range args {
		out, err := exec.CommandContext(ctx, "mount", a...).CombinedOutput()
		if err == nil {
			lastErr = nil
			break
		}
		lastErr = fmt.Errorf("mount: %w: %s", err, strings.TrimSpace(string(out)))
	}
	if lastErr != nil {
		_ = os.Remove(target)
		return Drive{}, lastErr
	}

	d.Mounted = true
	d.MountPath = target
	return d, nil
}

// mountArgs returns one or more candidate argv sets to try in order for a
// given filesystem type, each as a full `mount` argv (no shell). Several
// filesystem types have more than one viable driver on a typical Linux
// box (in particular ntfs), so the caller tries each until one succeeds.
// nosuid,nodev on every mount: media from an unknown source shouldn't be
// able to hand out setuid binaries or device nodes to anything that reads
// it, including whatever container later bind-mounts this path.
func mountArgs(fstype, device, target string) ([][]string, error) {
	base := []string{"-o", "nosuid,nodev"}
	switch fstype {
	case "ext4", "ext3", "ext2", "xfs", "btrfs", "vfat", "exfat":
		return [][]string{append(append([]string{"-t", fstype}, base...), device, target)}, nil
	case "ntfs":
		// Prefer the in-kernel ntfs3 driver (Linux 5.15+); fall back to the
		// ntfs-3g FUSE driver on an older kernel or if it's not built in.
		return [][]string{
			append(append([]string{"-t", "ntfs3"}, base...), device, target),
			append(append([]string{"-t", "ntfs-3g"}, base...), device, target),
		}, nil
	case "":
		return nil, fmt.Errorf("unknown filesystem type")
	default:
		// Anything else lsblk reported a name for but this switch doesn't
		// special-case: pass it straight through as -t <fstype> rather than
		// refusing outright. Not auto-detection (lsblk already told us the
		// type); this just trusts that answer instead of hand-listing every
		// filesystem `mount` supports.
		return [][]string{append([]string{"-t", fstype}, append(base, device, target)...)}, nil
	}
}

func (s *Server) unmount(ctx context.Context, uuid string) error {
	target := filepath.Join(s.MountRoot, uuid)
	if !strings.HasPrefix(target, filepath.Clean(s.MountRoot)+string(filepath.Separator)) {
		return fmt.Errorf("refusing to unmount outside mount root")
	}
	if _, err := os.Stat(target); err != nil {
		return fmt.Errorf("not mounted here: %w", err)
	}
	out, err := exec.CommandContext(ctx, "umount", target).CombinedOutput()
	if err != nil {
		return fmt.Errorf("umount: %w: %s", err, strings.TrimSpace(string(out)))
	}
	_ = os.Remove(target)
	return nil
}
