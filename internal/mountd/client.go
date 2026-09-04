package mountd

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"time"
)

// ErrUnavailable means the helper's socket couldn't be reached at all
// (not installed, not running, or this box has no storage feature set up)
// -- distinct from a Response.Error, which means the helper is up but
// refused or failed a specific request. Callers use this to degrade
// gracefully (e.g. the dashboard's Storage page shows "not set up on this
// box" instead of a scary error) rather than treating every box without
// mangrove-mountd as broken.
var ErrUnavailable = errors.New("storage helper (mangrove-mountd) is not reachable")

// Client talks to a running mangrove-mountd over its Unix domain socket.
// Stateless and safe for concurrent use -- each call dials fresh.
type Client struct {
	SocketPath string
}

func NewClient(socketPath string) *Client {
	return &Client{SocketPath: socketPath}
}

func (c *Client) call(ctx context.Context, req Request) (Response, error) {
	var d net.Dialer
	conn, err := d.DialContext(ctx, "unix", c.SocketPath)
	if err != nil {
		return Response{}, ErrUnavailable
	}
	defer conn.Close()

	if deadline, ok := ctx.Deadline(); ok {
		conn.SetDeadline(deadline)
	} else {
		conn.SetDeadline(time.Now().Add(30 * time.Second))
	}

	if err := json.NewEncoder(conn).Encode(req); err != nil {
		return Response{}, fmt.Errorf("write request: %w", err)
	}
	var resp Response
	if err := json.NewDecoder(bufio.NewReader(conn)).Decode(&resp); err != nil {
		return Response{}, fmt.Errorf("read response: %w", err)
	}
	if !resp.OK {
		return Response{}, fmt.Errorf("%s", resp.Error)
	}
	return resp, nil
}

// List returns every removable drive the helper is willing to consider,
// mounted or not.
func (c *Client) List(ctx context.Context) ([]Drive, error) {
	resp, err := c.call(ctx, Request{Action: ActionList})
	if err != nil {
		return nil, err
	}
	return resp.Drives, nil
}

// Mount asks the helper to mount the drive with this filesystem UUID under
// its own controlled root and returns the resulting Drive (with MountPath
// set).
func (c *Client) Mount(ctx context.Context, uuid string) (Drive, error) {
	resp, err := c.call(ctx, Request{Action: ActionMount, UUID: uuid})
	if err != nil {
		return Drive{}, err
	}
	if resp.Drive == nil {
		return Drive{}, fmt.Errorf("mountd: mount succeeded but returned no drive")
	}
	return *resp.Drive, nil
}

// Unmount asks the helper to unmount a drive it previously mounted. The
// caller is responsible for making sure nothing is still using its
// MountPath (e.g. a running NAS-share container) before calling this --
// the helper itself doesn't know about Mangrove's deployments.
func (c *Client) Unmount(ctx context.Context, uuid string) error {
	_, err := c.call(ctx, Request{Action: ActionUnmount, UUID: uuid})
	return err
}
