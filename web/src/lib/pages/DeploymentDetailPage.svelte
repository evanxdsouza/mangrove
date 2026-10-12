<script lang="ts">
  import { api, type Deployment, type DeployHistory, type Service } from "../api";
  import { errMsg } from "../errors";
  import { firstTime } from "../milestones";
  import { showToast } from "../stores/toast.svelte";
  import { workspaceRole } from "../stores/workspaces.svelte";
  import Icon from "../components/Icon.svelte";
  import StatusPill from "../components/StatusPill.svelte";
  import TideGauge from "../components/TideGauge.svelte";
  import ConfirmModal from "../components/ConfirmModal.svelte";
  import DomainsPanel from "../components/DomainsPanel.svelte";
  import DeployTimeline from "../components/DeployTimeline.svelte";
  import DeployHistorySparkline from "../components/DeployHistorySparkline.svelte";
  import LogViewer from "../components/LogViewer.svelte";
  import EnvVarsEditor from "../components/EnvVarsEditor.svelte";
  import OverviewTab from "../components/deployment/OverviewTab.svelte";
  import ServiceStatRow from "../components/deployment/ServiceStatRow.svelte";
  import AccessControlCard from "../components/deployment/AccessControlCard.svelte";
  import SleepCard from "../components/deployment/SleepCard.svelte";
  import SelfHealCard from "../components/deployment/SelfHealCard.svelte";
  import AutoDeployCard from "../components/deployment/AutoDeployCard.svelte";
  import PromoteCard from "../components/deployment/PromoteCard.svelte";
  import StagingCard from "../components/deployment/StagingCard.svelte";
  import PreviewsCard from "../components/deployment/PreviewsCard.svelte";

  let { projectId, deploymentId }: { projectId: number; deploymentId: number } = $props();

  type Tab = "overview" | "history" | "logs" | "env";

  let projectWorkspaceId = $state<number | null>(null);
  let projectName = $state<string | null>(null);
  let deployment = $state<Deployment | null>(null);
  let services = $state<Service[]>([]);
  let history = $state<DeployHistory[]>([]);
  let error = $state<string | null>(null);
  let tab = $state<Tab>("overview");
  let deploying = $state(false);
  let canceling = $state(false);
  let redeploying = $state(false);
  let stopping = $state(false);
  let restarting = $state(false);
  let rollbackBusyId = $state<number | null>(null);
  let selectedServiceId = $state<number | null>(null);
  let showDelete = $state(false);
  let burstKey = $state<number | undefined>(undefined);
  let prevStatus: string | null = null;

  const role = $derived(workspaceRole(projectWorkspaceId));
  const isAdmin = $derived(role === "admin");
  const canEdit = $derived(role === "admin" || role === "editor");

  function load() {
    api
      .get<Deployment>(`/api/deployments/${deploymentId}`)
      .then((d) => {
        // A fresh transition into "running" is real, observable feedback --
        // the gauge pops, and (once, ever, per browser) the very first one
        // anywhere gets a milestone toast.
        if (d.status === "running" && prevStatus != null && prevStatus !== "running") {
          burstKey = Date.now();
          if (firstTime("first-deploy")) showToast("First deploy live. This is the start of something growing.");
        }
        prevStatus = d.status;
        deployment = d;
        error = null;
      })
      .catch((e) => (error = errMsg(e)));
    api
      .get<Service[]>(`/api/deployments/${deploymentId}/services`)
      .then((s) => {
        services = s ?? [];
        if (selectedServiceId == null && s && s.length > 0) selectedServiceId = s[0].id;
        error = null;
      })
      .catch((e) => (error = errMsg(e)));
    api
      .get<DeployHistory[]>(`/api/deployments/${deploymentId}/history`)
      .then((h) => (history = h ?? []))
      .catch((e) => (error = errMsg(e)));
  }

  $effect(() => {
    void deploymentId;
    load();
    const interval = setInterval(load, 4000);
    return () => clearInterval(interval);
  });

  // Resolved once per project, not on every poll -- role gating on this
  // page is workspace-scoped, but a Deployment doesn't carry workspace_id.
  $effect(() => {
    api
      .get<{ workspace_id: number; name: string }>(`/api/projects/${projectId}`)
      .then((p) => {
        projectWorkspaceId = p.workspace_id;
        projectName = p.name;
      })
      .catch(() => {});
  });

  async function act(path: string, setBusy: (v: boolean) => void) {
    setBusy(true);
    error = null;
    try {
      await api.post(`/api/deployments/${deploymentId}/${path}`, {});
      load();
    } catch (e) {
      error = errMsg(e);
    } finally {
      setBusy(false);
    }
  }
  const deploy = () => act("deploy", (v) => (deploying = v));
  const cancel = () => act("cancel", (v) => (canceling = v));
  const redeploy = () => act("redeploy", (v) => (redeploying = v));
  const stop = () => act("stop", (v) => (stopping = v));
  const restart = () => act("restart", (v) => (restarting = v));

  async function rollback(historyId: number) {
    rollbackBusyId = historyId;
    error = null;
    try {
      await api.post(`/api/deploy-history/${historyId}/rollback`, {});
      if (firstTime("first-rollback")) showToast("First rollback complete. Good instinct, keeping a healthy version one click away.");
      load();
    } catch (e) {
      error = errMsg(e);
    } finally {
      rollbackBusyId = null;
    }
  }

  // A static site has no container to stream logs from -- Caddy serves the
  // built files directly.
  const tabs = $derived((["overview", "history", "logs", "env"] as Tab[]).filter((t) => t !== "logs" || deployment?.build_strategy !== "static"));
</script>

<nav class="breadcrumb" aria-label="Breadcrumb">
  <a href="/" class="breadcrumb-back"><Icon name="chevron-down" /> Projects</a>
  <span>/</span>
  <a href="/projects/{projectId}">{projectName ?? "..."}</a>
  <span>/</span>
  <span>{deployment?.name ?? "..."}</span>
</nav>

<section class="card deployment-header">
  <div class="deployment-header-id">
    <TideGauge status={deployment?.status} size="lg" {burstKey} />
    <div style="min-width: 0">
      <div class="deployment-header-title">
        <h1>{deployment?.name ?? "Loading..."}</h1>
        {#if deployment}<StatusPill status={deployment.status} />{/if}
      </div>
      {#if deployment}
        <div class="deployment-header-meta">
          <span class="mono">{deployment.build_strategy}</span>
          <span class="mono">{deployment.slug}</span>
          {#if deployment.environment === "staging"}<span class="pill pill-yellow">staging</span>{/if}
          {#if deployment.environment === "preview"}
            <span class="pill pill-gray">preview{deployment.pr_number ? ` · PR #${deployment.pr_number}` : ""}</span>
          {/if}
          {#if deployment.is_public && deployment.password_protected}
            <span><Icon name="lock" /> Password-protected</span>
          {:else if deployment.is_public}
            <span><Icon name="globe" /> Public</span>
          {/if}
        </div>
      {/if}
    </div>
  </div>
  <div class="deployment-header-actions">
    {#if deployment?.status === "building" || deployment?.status === "pending"}
      <button class="btn btn-danger" onclick={cancel} disabled={canceling}>{canceling ? "Cancelling..." : "Cancel deploy"}</button>
    {:else}
      <button class="btn btn-primary" onclick={deploy} disabled={deploying}>{deploying ? "Deploying..." : "Deploy"}</button>
    {/if}
    <button
      class="btn"
      onclick={redeploy}
      disabled={redeploying}
      title="Rebuild and redeploy from the currently configured source (linked repo branch, or image ref)"
    >
      {redeploying ? "Redeploying..." : "Redeploy"}
    </button>
    {#if deployment?.build_strategy !== "static"}
      {#if deployment?.status === "stopped"}
        <button class="btn" onclick={restart} disabled={restarting}>{restarting ? "Starting..." : "Start"}</button>
      {:else if deployment?.status === "sleeping"}
        <button class="btn" onclick={restart} disabled={restarting} title="Wake it up now instead of waiting for the next visitor to trigger it">
          {restarting ? "Waking up..." : "Wake up"}
        </button>
      {:else}
        <button class="btn" onclick={restart} disabled={restarting}>{restarting ? "Restarting..." : "Restart"}</button>
        <button class="btn" onclick={stop} disabled={stopping}>{stopping ? "Stopping..." : "Stop"}</button>
      {/if}
    {/if}
    {#if isAdmin}
      <button class="btn btn-ghost-danger" onclick={() => (showDelete = true)}><Icon name="trash" /> Delete deployment</button>
    {/if}
  </div>
</section>

{#if error}<div class="error-banner">{error}</div>{/if}

<!-- The single-service stat strip; a compose deployment has no one
     canonical service, so it keeps per-service cards in Overview instead. -->
{#if services.length === 1}
  <ServiceStatRow service={services[0]} isStatic={deployment?.build_strategy === "static"} />
{/if}

<div class="tabs" role="tablist" aria-label="Deployment sections">
  {#each tabs as t (t)}
    <button type="button" role="tab" aria-selected={tab === t} class="tab {tab === t ? 'active' : ''}" onclick={() => (tab = t)}>
      {t[0].toUpperCase() + t.slice(1)}
    </button>
  {/each}
</div>

{#if tab === "overview"}
  <div class="overview-grid">
    <OverviewTab {services} {deployment} {canEdit} />
    <AccessControlCard {deploymentId} {deployment} onSaved={load} {isAdmin} />
    <SleepCard {deploymentId} {deployment} onSaved={load} {canEdit} />
    <SelfHealCard {deploymentId} {deployment} onSaved={load} {canEdit} />
    <DomainsPanel {deploymentId} {canEdit} {isAdmin} />
    <AutoDeployCard {projectId} {deploymentId} {deployment} onSaved={load} />
    {#if deployment?.promotes_to_deployment_id != null}
      <PromoteCard {projectId} {deploymentId} {deployment} />
    {:else if deployment}
      <StagingCard {projectId} productionDeployment={deployment} />
      <PreviewsCard {projectId} productionDeployment={deployment} />
    {/if}
  </div>
{:else if tab === "history"}
  <div class="card">
    <DeployHistorySparkline {history} />
    <DeployTimeline {history} onRollback={rollback} busyId={rollbackBusyId} {projectId} {deploymentId} />
  </div>
{:else if tab === "logs"}
  <div class="card">
    {#if services.length === 0}
      <div class="empty-state"><p>No services yet.</p></div>
    {:else}
      {#if services.length > 1}
        <div class="field">
          <label for="log-service-select">Service</label>
          <select id="log-service-select" class="input" bind:value={selectedServiceId}>
            {#each services as s (s.id)}<option value={s.id}>{s.name}</option>{/each}
          </select>
        </div>
      {/if}
      {#if selectedServiceId}
        {#key selectedServiceId}<LogViewer serviceId={selectedServiceId} />{/key}
      {/if}
    {/if}
  </div>
{:else}
  <div class="card">
    {#if services.length === 0}
      <div class="empty-state"><p>No services yet.</p></div>
    {:else}
      {#each services as s (s.id)}
        <div style="margin-bottom: 20px">
          {#if services.length > 1}<div class="card-title">{s.name}</div>{/if}
          <EnvVarsEditor serviceId={s.id} {isAdmin} />
        </div>
      {/each}
    {/if}
  </div>
{/if}

{#if showDelete}
  <ConfirmModal
    title="Delete deployment"
    body={`This permanently deletes "${deployment?.name ?? "this deployment"}" -- stopping its container(s), removing volumes, and releasing its port. This cannot be undone.`}
    confirmLabel="Delete deployment"
    onClose={() => (showDelete = false)}
    onConfirm={async () => {
      await api.del(`/api/deployments/${deploymentId}`);
      window.location.href = `/projects/${projectId}`;
    }}
  />
{/if}
