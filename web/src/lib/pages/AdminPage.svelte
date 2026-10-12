<script lang="ts">
  import { api, type AuditEvent, type ResourceUsageSnapshot } from "../api";
  import { errMsg } from "../errors";
  import { fmtWhen, humanLabel } from "../format";
  import { isOwner } from "../stores/user.svelte";
  import Icon from "../components/Icon.svelte";
  import StatTile from "../components/StatTile.svelte";
  import ResourceHistoryStrip from "../components/ResourceHistoryStrip.svelte";
  import AuditLogTable from "../components/AuditLogTable.svelte";
  import AddTeamUserForm from "../components/admin/AddTeamUserForm.svelte";
  import AddPatForm from "../components/admin/AddPatForm.svelte";
  import ReservePortForm from "../components/admin/ReservePortForm.svelte";

  interface ResourceBudget {
    memory_allocated_mb: number;
    memory_used_mb: number;
    memory_ceiling_mb: number;
    running_containers: number;
    disk_total_gb: number;
    disk_used_gb: number;
  }
  interface PortEntry {
    id: number;
    port: number;
    status: string;
    allocation_type: string;
    service_id?: number;
    note: string;
  }
  interface SessionEntry {
    id: number;
    user_email: string;
    created_at: string;
    expires_at: string;
    ip_address: string;
    user_agent: string;
  }
  interface NodeEntry {
    id: number;
    name: string;
    kind: string;
    address?: string;
    status: string;
    is_default: boolean;
  }
  interface NotificationEntry {
    id: number;
    type: string;
    channel: string;
    recipient: string;
    subject: string;
    status: string;
    sent_at: string;
  }
  interface GithubPATEntry {
    id: number;
    label: string;
    created_at: string;
    last_used_at?: string;
  }
  interface TeamUserEntry {
    id: number;
    email: string;
    role: "owner" | "member";
    created_at: string;
    last_login_at?: string;
  }

  const owner = isOwner();
  let budget = $state<ResourceBudget | null>(null);
  let historyHours = $state(24);
  let history = $state<ResourceUsageSnapshot[] | null>(null);
  let ports = $state<PortEntry[] | null>(null);
  let sessions = $state<SessionEntry[] | null>(null);
  let nodes = $state<NodeEntry[] | null>(null);
  let notifications = $state<NotificationEntry[] | null>(null);
  let pats = $state<GithubPATEntry[] | null>(null);
  let users = $state<TeamUserEntry[] | null>(null);
  let auditLog = $state<AuditEvent[] | null>(null);
  let error = $state<string | null>(null);
  let pruneResult = $state<string | null>(null);

  const fail = (e: unknown) => (error = errMsg(e));

  function load() {
    api.get<ResourceBudget>("/api/admin/resource-budget").then((b) => (budget = b)).catch(fail);
    api.get<NodeEntry[]>("/api/admin/nodes").then((n) => (nodes = n ?? [])).catch(fail);
    api.get<NotificationEntry[]>("/api/admin/notifications").then((n) => (notifications = n ?? [])).catch(fail);
    api.get<GithubPATEntry[]>("/api/github/pats").then((p) => (pats = p ?? [])).catch(fail);
    // Ports, sessions, users, and the audit log are owner-only server-side
    // (docs/multi-user.md) -- fetching them as a member would just 403 and
    // show a spurious error banner on a page they can otherwise use.
    if (owner) {
      api.get<PortEntry[]>("/api/admin/ports").then((p) => (ports = p ?? [])).catch(fail);
      api.get<SessionEntry[]>("/api/admin/sessions").then((s) => (sessions = s ?? [])).catch(fail);
      api.get<TeamUserEntry[]>("/api/admin/users").then((u) => (users = u ?? [])).catch(fail);
      api.get<AuditEvent[]>("/api/admin/audit-log").then((a) => (auditLog = a ?? [])).catch(fail);
    }
  }
  load();

  $effect(() => {
    api
      .get<ResourceUsageSnapshot[]>(`/api/admin/resource-history?hours=${historyHours}`)
      .then((h) => (history = h ?? []))
      .catch(fail);
  });

  async function removeUser(id: number) {
    error = null;
    try {
      await api.del(`/api/admin/users/${id}`);
      load();
    } catch (e) {
      // Real expected failure modes here (deleting yourself, the last owner).
      error = errMsg(e);
    }
  }

  async function revokeSession(id: number) {
    await api.del(`/api/admin/sessions/${id}`);
    load();
  }

  async function releasePort(port: number) {
    await api.del(`/api/admin/ports/${port}`);
    load();
  }

  async function deletePat(id: number) {
    await api.del(`/api/github/pats/${id}`);
    load();
  }

  async function runPrune() {
    pruneResult = null;
    try {
      const res = await api.post<{ images_removed: number; space_reclaimed_mb: number }>("/api/admin/prune");
      pruneResult = `Removed ${res.images_removed} image(s), reclaimed ~${res.space_reclaimed_mb}MB.`;
      load();
    } catch (e) {
      error = errMsg(e);
    }
  }
</script>

<div class="page-header">
  <div>
    <h1>Admin</h1>
    <p>System-wide operations across every project.</p>
  </div>
  <Icon name="dials" />
</div>

{#if error}<div class="error-banner">{error}</div>{/if}

<div class="card">
  <div class="card-title">Resource budget</div>
  {#if budget}
    <div class="instrument-row">
      <StatTile
        label="Memory allocated"
        value="{budget.memory_allocated_mb} / {budget.memory_ceiling_mb} MB"
        fraction={budget.memory_allocated_mb / Math.max(budget.memory_ceiling_mb, 1)}
      />
      <StatTile label="Memory used" value="{budget.memory_used_mb.toFixed(0)} MB" fraction={budget.memory_used_mb / Math.max(budget.memory_ceiling_mb, 1)} />
      <div class="instrument-cell">
        <div class="stat-tile">
          <div class="stat-value">{budget.running_containers}</div>
          <div class="stat-label">Running containers</div>
        </div>
      </div>
      {#if budget.disk_total_gb > 0}
        <StatTile
          label="Disk used"
          value="{budget.disk_used_gb.toFixed(1)} / {budget.disk_total_gb.toFixed(1)} GB"
          fraction={budget.disk_used_gb / Math.max(budget.disk_total_gb, 1)}
        />
      {/if}
    </div>
  {:else}
    <div class="text-dim">Loading...</div>
  {/if}

  {#if history}
    <div style="margin-top: 18px">
      <div class="flex-between" style="margin-bottom: 8px">
        <div class="field-hint" style="margin: 0">Trend -- sampled every 5 minutes, kept for 30 days</div>
        <select class="input" style="width: auto" aria-label="Trend window" bind:value={historyHours}>
          <option value={24}>Last 24 hours</option>
          <option value={168}>Last 7 days</option>
          <option value={720}>Last 30 days</option>
        </select>
      </div>
      <div class="field-hint" style="margin-bottom: 4px">Memory used</div>
      <ResourceHistoryStrip snapshots={history} metric="memory" />
      <div class="field-hint" style="margin: 12px 0 4px">Disk used</div>
      <ResourceHistoryStrip snapshots={history} metric="disk" />
      <!-- Every chart explains its own colors. -->
      <div class="chart-legend">
        <span class="chart-legend-item"><span class="chart-legend-swatch" style="background: var(--tide-high)"></span> Normal</span>
        <span class="chart-legend-item"><span class="chart-legend-swatch" style="background: var(--ochre)"></span> Above 70%</span>
        <span class="chart-legend-item"><span class="chart-legend-swatch" style="background: var(--red)"></span> Above 90%</span>
      </div>
    </div>
  {/if}
</div>

<!-- Port registry and sessions are owner-only server-side, so they're
     owner-only here too -- a member would just hit 403s on every button. -->
{#if owner}
  <div class="card">
    <div class="card-title">Port registry</div>
    {#if ports === null}
      <div class="text-dim">Loading...</div>
    {:else if ports.length === 0}
      <div class="text-dim">No ports allocated yet.</div>
    {:else}
      <table>
        <thead><tr><th>Port</th><th>Type</th><th>Note</th><th></th></tr></thead>
        <tbody>
          {#each ports as p (p.id)}
            <tr>
              <td class="mono">{p.port}</td>
              <td class="text-dim">{humanLabel(p.allocation_type)}</td>
              <td class="text-dim">{p.note || "—"}</td>
              <td>
                {#if p.allocation_type !== "system"}
                  <button class="btn btn-sm btn-danger" onclick={() => releasePort(p.port)}><Icon name="trash" /> Release</button>
                {/if}
              </td>
            </tr>
          {/each}
        </tbody>
      </table>
    {/if}
    <ReservePortForm onReserved={load} />
  </div>

  <div class="card">
    <div class="card-title">Active sessions</div>
    {#if sessions === null}
      <div class="text-dim">Loading...</div>
    {:else if sessions.length === 0}
      <div class="text-dim">No active sessions.</div>
    {:else}
      <table>
        <thead><tr><th>User</th><th>IP</th><th>Created</th><th></th></tr></thead>
        <tbody>
          {#each sessions as s (s.id)}
            <tr>
              <td>{s.user_email}</td>
              <td class="mono text-dim">{s.ip_address}</td>
              <td class="text-dim">{fmtWhen(s.created_at)}</td>
              <td><button class="btn btn-sm btn-danger" onclick={() => revokeSession(s.id)}><Icon name="trash" /> Revoke</button></td>
            </tr>
          {/each}
        </tbody>
      </table>
    {/if}
  </div>
{/if}

<div class="grid grid-2">
  <div class="card">
    <div class="card-title">Nodes</div>
    {#if nodes === null}
      <div class="text-dim">Loading...</div>
    {:else}
      <table>
        <thead><tr><th>Name</th><th>Kind</th><th>Status</th></tr></thead>
        <tbody>
          {#each nodes as n (n.id)}
            <tr>
              <td>{n.name} {#if n.is_default}<span class="pill pill-gray">default</span>{/if}</td>
              <td class="text-dim">{n.kind}</td>
              <td><span class="pill pill-green"><span class="pill-dot"></span>{n.status}</span></td>
            </tr>
          {/each}
        </tbody>
      </table>
    {/if}
    <div class="field-hint" style="margin-top: 12px">Single-node only for now. A future remote worker would appear here.</div>
  </div>

  {#if owner}
    <div class="card">
      <div class="card-title">Image / build cache pruning</div>
      <p class="text-dim" style="margin-top: 0">Runs automatically on a schedule; trigger it manually here too.</p>
      <button class="btn" onclick={runPrune}>Prune now</button>
      {#if pruneResult}<div class="field-hint">{pruneResult}</div>{/if}
    </div>
  {/if}
</div>

{#if owner}
  <div class="card">
    <div class="card-title">Team</div>
    <p class="text-dim" style="margin-top: 0">
      Owners can delete projects/deployments, manage the team, view/set secrets, and change access control. Members can do
      everything else.
    </p>
    {#if users === null}
      <div class="text-dim">Loading...</div>
    {:else}
      <table>
        <thead><tr><th>Email</th><th>Role</th><th>Last login</th><th></th></tr></thead>
        <tbody>
          {#each users as u (u.id)}
            <tr>
              <td>{u.email}</td>
              <td class="text-dim">{u.role}</td>
              <td class="text-dim">{u.last_login_at ? fmtWhen(u.last_login_at) : "never"}</td>
              <td><button class="btn btn-sm btn-danger" onclick={() => removeUser(u.id)}><Icon name="trash" /> Remove</button></td>
            </tr>
          {/each}
        </tbody>
      </table>
    {/if}
    <AddTeamUserForm onAdded={load} />
  </div>
{/if}

<div class="card">
  <div class="card-title">GitHub Personal Access Tokens</div>
  <p class="text-dim" style="margin-top: 0">
    Used to clone private repos for git-backed deployments. Stored encrypted; the token itself is never shown again after creation.
  </p>
  {#if pats === null}
    <div class="text-dim">Loading...</div>
  {:else if pats.length === 0}
    <div class="text-dim">No tokens added yet.</div>
  {:else}
    <table>
      <thead><tr><th>Label</th><th>Added</th><th>Last used</th><th></th></tr></thead>
      <tbody>
        {#each pats as p (p.id)}
          <tr>
            <td>{p.label}</td>
            <td class="text-dim">{fmtWhen(p.created_at)}</td>
            <td class="text-dim">{p.last_used_at ? fmtWhen(p.last_used_at) : "never"}</td>
            <td><button class="btn btn-sm btn-danger" onclick={() => deletePat(p.id)}><Icon name="trash" /> Remove</button></td>
          </tr>
        {/each}
      </tbody>
    </table>
  {/if}
  <AddPatForm onAdded={load} />
</div>

<div class="card">
  <div class="card-title">Notifications</div>
  {#if notifications === null}
    <div class="text-dim">Loading...</div>
  {:else if notifications.length === 0}
    <div class="text-dim">No notifications sent yet.</div>
  {:else}
    <table>
      <thead><tr><th>Type</th><th>Recipient</th><th>Status</th><th>Sent</th></tr></thead>
      <tbody>
        {#each notifications as n (n.id)}
          <tr>
            <td>{n.subject}</td>
            <td class="text-dim">{n.recipient}</td>
            <td><span class="pill {n.status === 'sent' ? 'pill-green' : 'pill-red'}"><span class="pill-dot"></span>{n.status}</span></td>
            <td class="text-dim">{fmtWhen(n.sent_at)}</td>
          </tr>
        {/each}
      </tbody>
    </table>
  {/if}
</div>

{#if owner}
  <div class="card">
    <div class="card-title">Audit log</div>
    <p class="text-dim" style="margin-top: 0">
      Every deploy, delete, and access/membership change across every workspace -- see docs/multi-user.md. A workspace's own
      members can see its own trail from the Workspaces page without needing this view.
    </p>
    <AuditLogTable events={auditLog} showWorkspace />
  </div>
{/if}
