import { useEffect, useState, type FormEvent } from "react";
import { api, ApiError, type Drive, type DrivesResponse, type NASShareInfo } from "../api";
import { useIsOwner } from "../userContext";
import { Link } from "../router";
import { Modal } from "../components/Modal";
import { slugify } from "./ProjectsPage";

function errMsg(e: unknown): string {
  return e instanceof ApiError ? e.message : "Something went wrong";
}

function formatSize(bytes: number): string {
  if (bytes <= 0) return "unknown size";
  const gb = bytes / 1e9;
  return gb >= 1 ? `${gb.toFixed(1)} GB` : `${(bytes / 1e6).toFixed(0)} MB`;
}

// A share's host_mount_source is always "<mountd root>/<uuid>" -- see
// internal/mountd.Server.mount -- so matching by suffix ties a drive to
// the share bind-mounting it without either side needing to know the
// other's exact path layout.
function shareForDrive(shares: NASShareInfo[], uuid: string): NASShareInfo | undefined {
  return shares.find((s) => s.host_mount_source.endsWith("/" + uuid));
}

export function StoragePage() {
  const isOwner = useIsOwner();

  if (!isOwner) {
    return (
      <div className="card empty-state">
        Storage/NAS sharing is an owner-only feature -- mounting drives and creating shares with plaintext credentials
        is system-level access, same bar as Admin.
      </div>
    );
  }

  return <StoragePageInner />;
}

function StoragePageInner() {
  const [drivesResp, setDrivesResp] = useState<DrivesResponse | null>(null);
  const [shares, setShares] = useState<NASShareInfo[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [busyUUID, setBusyUUID] = useState<string | null>(null);
  const [shareTarget, setShareTarget] = useState<Drive | null>(null);

  const load = () => {
    Promise.all([api.get<DrivesResponse>("/api/storage/drives"), api.get<NASShareInfo[]>("/api/storage/shares")])
      .then(([d, s]) => {
        setDrivesResp(d);
        setShares(s);
      })
      .catch((e) => setError(errMsg(e)));
  };

  useEffect(load, []);

  const mount = async (uuid: string) => {
    setBusyUUID(uuid);
    setError(null);
    try {
      await api.post(`/api/storage/drives/${uuid}/mount`);
      load();
    } catch (e) {
      setError(errMsg(e));
    } finally {
      setBusyUUID(null);
    }
  };

  const unmount = async (uuid: string) => {
    setBusyUUID(uuid);
    setError(null);
    try {
      await api.post(`/api/storage/drives/${uuid}/unmount`);
      load();
    } catch (e) {
      setError(errMsg(e));
    } finally {
      setBusyUUID(null);
    }
  };

  if (drivesResp === null || shares === null) {
    return (
      <div className="center-loading">
        <div className="spinner" />
      </div>
    );
  }

  return (
    <div>
      <div className="page-header">
        <h1>Storage</h1>
      </div>
      <p className="text-dim" style={{ marginTop: 0 }}>
        Turn a plugged-in drive into a network share (SMB) other devices on your LAN can connect to. Requires the
        mangrove-mountd helper -- see docs/storage.md for how to install it.
      </p>
      {error && <div className="error-banner">{error}</div>}

      {!drivesResp.helper_available ? (
        <div className="card empty-state">
          The storage helper (mangrove-mountd) isn't reachable on this box. It's a separate, optional component --
          see docs/storage.md for what it does and how to install it.
        </div>
      ) : drivesResp.drives.length === 0 ? (
        <div className="card empty-state">No removable drives detected. Plug one in, then reload this page.</div>
      ) : (
        <div className="grid grid-2">
          {drivesResp.drives.map((d) => {
            const share = shareForDrive(shares, d.uuid);
            const busy = busyUUID === d.uuid;
            return (
              <div className="card" key={d.uuid}>
                <div className="card-title">{d.label || d.device}</div>
                <div className="kv-list">
                  <div className="kv-row">
                    <span className="kv-key">Device</span>
                    <span className="kv-value mono">{d.device}</span>
                  </div>
                  <div className="kv-row">
                    <span className="kv-key">Size</span>
                    <span className="kv-value">{formatSize(d.size_bytes)}</span>
                  </div>
                  <div className="kv-row">
                    <span className="kv-key">Filesystem</span>
                    <span className="kv-value">{d.filesystem || "unknown"}</span>
                  </div>
                  <div className="kv-row">
                    <span className="kv-key">Status</span>
                    <span className="kv-value">
                      {share
                        ? `Shared as "${share.share_name}"`
                        : d.mounted
                          ? "Mounted"
                          : "Not mounted"}
                    </span>
                  </div>
                </div>
                <div className="flex gap-8" style={{ marginTop: 12 }}>
                  {!d.mounted && (
                    <button className="btn btn-primary btn-sm" disabled={busy} onClick={() => mount(d.uuid)}>
                      {busy ? "Mounting..." : "Mount"}
                    </button>
                  )}
                  {d.mounted && !share && (
                    <>
                      <button className="btn btn-primary btn-sm" onClick={() => setShareTarget(d)}>
                        Share as NAS
                      </button>
                      <button className="btn btn-sm" disabled={busy} onClick={() => unmount(d.uuid)}>
                        {busy ? "Unmounting..." : "Unmount"}
                      </button>
                    </>
                  )}
                  {share && (
                    <Link to={`/projects/1/deployments/${share.deployment_id}`} className="btn btn-sm">
                      Manage share
                    </Link>
                  )}
                </div>
              </div>
            );
          })}
        </div>
      )}

      {shares.length > 0 && (
        <>
          <div className="card-title" style={{ marginTop: 24 }}>
            Active shares
          </div>
          <div className="kv-list">
            {shares.map((s) => (
              <div className="kv-row" key={s.service_id}>
                <span className="kv-key">{s.share_name}</span>
                <span className="kv-value mono">
                  smb://{window.location.hostname}/{s.share_name} &middot; {s.status}
                </span>
              </div>
            ))}
          </div>
          <div className="field-hint" style={{ marginTop: 8 }}>
            To stop sharing, delete the share's deployment from its own page -- a NAS share doesn't support
            redeploy/rollback/scale (it holds an exclusive network port for as long as it runs), so deleting and
            re-sharing is how its configuration changes.
          </div>
        </>
      )}

      {shareTarget && (
        <ShareDriveForm
          drive={shareTarget}
          onClose={() => setShareTarget(null)}
          onShared={() => {
            setShareTarget(null);
            load();
          }}
        />
      )}
    </div>
  );
}

function randomPassword(): string {
  const bytes = new Uint8Array(12);
  crypto.getRandomValues(bytes);
  return Array.from(bytes, (b) => b.toString(36).padStart(2, "0")).join("").slice(0, 16);
}

function ShareDriveForm({ drive, onClose, onShared }: { drive: Drive; onClose: () => void; onShared: () => void }) {
  const defaultName = drive.label || drive.device.replace("/dev/", "");
  const [shareName, setShareName] = useState(defaultName);
  const [slug, setSlug] = useState(slugify(defaultName));
  const [username, setUsername] = useState("share");
  const [password, setPassword] = useState(randomPassword());
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  const submit = async (e: FormEvent) => {
    e.preventDefault();
    setBusy(true);
    setError(null);
    try {
      await api.post("/api/storage/shares", {
        drive_uuid: drive.uuid,
        slug,
        share_name: shareName,
        username,
        password,
      });
      onShared();
    } catch (e) {
      setError(errMsg(e));
    } finally {
      setBusy(false);
    }
  };

  return (
    <Modal title={`Share ${drive.label || drive.device} as NAS`} onClose={onClose}>
      {error && <div className="error-banner">{error}</div>}
      <form onSubmit={submit}>
        <div className="field">
          <label htmlFor="nas-share-name">Share name</label>
          <input
            id="nas-share-name"
            className="input"
            required
            value={shareName}
            onChange={(e) => {
              setShareName(e.target.value);
              setSlug(slugify(e.target.value));
            }}
          />
          <div className="field-hint">
            Shown as smb://{window.location.hostname}/{shareName || "..."}
          </div>
        </div>
        <div className="field">
          <label htmlFor="nas-slug">Slug</label>
          <input id="nas-slug" className="input mono" required value={slug} onChange={(e) => setSlug(slugify(e.target.value))} />
        </div>
        <div className="field">
          <label htmlFor="nas-username">SMB username</label>
          <input id="nas-username" className="input" required value={username} onChange={(e) => setUsername(e.target.value)} />
        </div>
        <div className="field">
          <label htmlFor="nas-password">SMB password</label>
          <input
            id="nas-password"
            className="input mono"
            required
            minLength={8}
            value={password}
            onChange={(e) => setPassword(e.target.value)}
          />
          <div className="field-hint">Shown once here -- write it down, Mangrove doesn't display it again.</div>
        </div>
        <div className="field-hint">
          The whole drive becomes writable over the network to anyone with these credentials. Creates a new
          deployment in the "Storage" project.
        </div>
        <div className="modal-actions">
          <button type="button" className="btn" onClick={onClose} disabled={busy}>
            Cancel
          </button>
          <button type="submit" className="btn btn-primary" disabled={busy}>
            {busy ? "Sharing..." : "Share"}
          </button>
        </div>
      </form>
    </Modal>
  );
}
