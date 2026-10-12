<script lang="ts">
  import { api, type Drive, type DrivesResponse, type NASShareInfo } from "../api";
  import { errMsg } from "../errors";
  import { isOwner } from "../stores/user.svelte";
  import Icon from "../components/Icon.svelte";
  import CenterLoading from "../components/CenterLoading.svelte";
  import ShareDriveForm from "../components/storage/ShareDriveForm.svelte";

  // Storage/NAS sharing mounts host drives and creates SMB shares with
  // plaintext credentials -- owner-only on the API, and here too.
  const owner = isOwner();

  function formatSize(bytes: number): string {
    if (bytes <= 0) return "unknown size";
    const gb = bytes / 1e9;
    return gb >= 1 ? `${gb.toFixed(1)} GB` : `${(bytes / 1e6).toFixed(0)} MB`;
  }

  // A share's host_mount_source is always "<mountd root>/<uuid>" (see
  // internal/mountd.Server.mount), so matching by suffix ties a drive to
  // the share bind-mounting it without either side knowing the other's
  // exact path layout.
  function shareForDrive(shares: NASShareInfo[], uuid: string): NASShareInfo | undefined {
    return shares.find((s) => s.host_mount_source.endsWith("/" + uuid));
  }

  let drivesResp = $state<DrivesResponse | null>(null);
  let shares = $state<NASShareInfo[] | null>(null);
  let error = $state<string | null>(null);
  let busyUUID = $state<string | null>(null);
  let shareTarget = $state<Drive | null>(null);

  function load() {
    error = null;
    Promise.all([api.get<DrivesResponse>("/api/storage/drives"), api.get<NASShareInfo[]>("/api/storage/shares")])
      .then(([d, s]) => {
        drivesResp = d;
        shares = s;
      })
      .catch((e) => (error = errMsg(e)));
  }
  if (owner) load();

  async function driveAction(uuid: string, action: "mount" | "unmount") {
    busyUUID = uuid;
    error = null;
    try {
      await api.post(`/api/storage/drives/${uuid}/${action}`);
      load();
    } catch (e) {
      error = errMsg(e);
    } finally {
      busyUUID = null;
    }
  }
</script>

{#if !owner}
  <div class="card empty-state">
    <Icon name="alert" />
    <p>Storage/NAS sharing is an owner-only feature.</p>
    <div class="field-hint">Mounting drives and creating shares with plaintext credentials is system-level access, same bar as Admin.</div>
  </div>
{:else if drivesResp === null || shares === null}
  {#if error}<div class="error-banner">{error}</div>{/if}
  <CenterLoading />
{:else}
  <div class="page-header">
    <div>
      <h1>Storage</h1>
      <p>
        Turn a plugged-in drive into a network share (SMB) other devices on your LAN can connect to. Requires the
        mangrove-mountd helper -- see docs/storage.md for how to install it.
      </p>
    </div>
    <Icon name="cabinet" />
  </div>
  {#if error}<div class="error-banner">{error}</div>{/if}

  {#if !drivesResp.helper_available}
    <div class="card empty-state">
      <p>The storage helper (mangrove-mountd) isn't reachable on this box.</p>
      <div class="field-hint">It's a separate, optional component -- see docs/storage.md for what it does and how to install it.</div>
    </div>
  {:else if drivesResp.drives.length === 0}
    <div class="card empty-state">
      <p>No removable drives detected.</p>
      <div class="field-hint">Plug one in, then reload this page.</div>
    </div>
  {:else}
    <div class="grid grid-2">
      {#each drivesResp.drives as d (d.uuid)}
        {@const share = shareForDrive(shares, d.uuid)}
        {@const busy = busyUUID === d.uuid}
        <div class="card">
          <div class="card-title">{d.label || d.device}</div>
          <div class="kv-list">
            <div class="kv-row"><span class="kv-key">Device</span><span class="kv-value mono">{d.device}</span></div>
            <div class="kv-row"><span class="kv-key">Size</span><span class="kv-value">{formatSize(d.size_bytes)}</span></div>
            <div class="kv-row"><span class="kv-key">Filesystem</span><span class="kv-value">{d.filesystem || "unknown"}</span></div>
            <div class="kv-row">
              <span class="kv-key">Status</span>
              <span class="kv-value">{share ? `Shared as "${share.share_name}"` : d.mounted ? "Mounted" : "Not mounted"}</span>
            </div>
          </div>
          <div class="flex gap-8" style="margin-top: 12px">
            {#if !d.mounted}
              <button class="btn btn-primary btn-sm" disabled={busy} onclick={() => driveAction(d.uuid, "mount")}>{busy ? "Mounting..." : "Mount"}</button>
            {/if}
            {#if d.mounted && !share}
              <button class="btn btn-primary btn-sm" onclick={() => (shareTarget = d)}>Share as NAS</button>
              <button class="btn btn-sm" disabled={busy} onclick={() => driveAction(d.uuid, "unmount")}>{busy ? "Unmounting..." : "Unmount"}</button>
            {/if}
            {#if share}
              <a href="/projects/{share.project_id}/deployments/{share.deployment_id}" class="btn btn-sm">Manage share</a>
            {/if}
          </div>
        </div>
      {/each}
    </div>
  {/if}

  {#if shares.length > 0}
    <div class="card-title" style="margin-top: 24px">Active shares</div>
    <div class="kv-list">
      {#each shares as s (s.service_id)}
        <div class="kv-row">
          <span class="kv-key">{s.share_name}</span>
          <span class="kv-value mono">smb://{window.location.hostname}/{s.share_name} &middot; {s.status}</span>
        </div>
      {/each}
    </div>
    <div class="field-hint" style="margin-top: 8px">
      To stop sharing, delete the share's deployment from its own page -- a NAS share doesn't support redeploy/rollback/scale
      (it holds an exclusive network port for as long as it runs), so deleting and re-sharing is how its configuration changes.
    </div>
  {/if}

  {#if shareTarget}
    <ShareDriveForm
      drive={shareTarget}
      onClose={() => (shareTarget = null)}
      onShared={() => {
        shareTarget = null;
        load();
      }}
    />
  {/if}
{/if}
