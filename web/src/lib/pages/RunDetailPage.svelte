<script lang="ts">
  import { api, type Deployment, type DeployHistory } from "../api";
  import { errMsg } from "../errors";
  import { fmtDuration, fmtWhenFull } from "../format";
  import { firstTime } from "../milestones";
  import { showToast } from "../stores/toast.svelte";
  import Icon from "../components/Icon.svelte";
  import StatusPill from "../components/StatusPill.svelte";
  import CodeBlock from "../components/CodeBlock.svelte";
  import CenterLoading from "../components/CenterLoading.svelte";

  // One run, its own page: breadcrumb back to the deployment, a header with
  // status/who/when/duration, then the run as a single labeled step block.
  // The data model has no discrete named build steps or persisted output
  // for a *successful* run (only error_message on failure), so this shows
  // one step rather than fabricating a breakdown or fake output.
  let { projectId, deploymentId, historyId }: { projectId: number; deploymentId: number; historyId: number } = $props();

  let projectName = $state<string | null>(null);
  let deployment = $state<Deployment | null>(null);
  let history = $state<DeployHistory[] | null>(null);
  let error = $state<string | null>(null);
  let rollbackBusy = $state(false);

  function load() {
    api
      .get<Deployment>(`/api/deployments/${deploymentId}`)
      .then((d) => (deployment = d))
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

  $effect(() => {
    api
      .get<{ name: string }>(`/api/projects/${projectId}`)
      .then((p) => (projectName = p.name))
      .catch(() => {});
  });

  const index = $derived(history?.findIndex((h) => h.id === historyId) ?? -1);
  const entry = $derived(history && index >= 0 ? history[index] : null);
  const runNumber = $derived(history && index >= 0 ? history.length - index : null);
  const duration = $derived(entry ? fmtDuration(entry.started_at, entry.finished_at) : "");
  const succeeded = $derived(entry?.status === "success");
  const failed = $derived(entry?.status === "failed" || entry?.status === "timeout");

  async function rollback() {
    if (!entry) return;
    rollbackBusy = true;
    error = null;
    try {
      await api.post(`/api/deploy-history/${entry.id}/rollback`, {});
      if (firstTime("first-rollback")) showToast("First rollback complete. Good instinct, keeping a healthy version one click away.");
      load();
    } catch (e) {
      error = errMsg(e);
    } finally {
      rollbackBusy = false;
    }
  }
</script>

{#snippet crumbs(name: string, last: string)}
  <nav class="breadcrumb" aria-label="Breadcrumb">
    <a href="/" class="breadcrumb-back"><Icon name="chevron-down" /> Projects</a>
    <span>/</span>
    <a href="/projects/{projectId}">{projectName ?? "..."}</a>
    <span>/</span>
    <a href="/projects/{projectId}/deployments/{deploymentId}">{name}</a>
    <span>/</span>
    <span>{last}</span>
  </nav>
{/snippet}

{#if history === null || deployment === null}
  <CenterLoading />
{:else if !entry}
  {@render crumbs(deployment.name, "Run")}
  <div class="card empty-state"><p>This run no longer exists -- deploy history is pruned over time.</p></div>
{:else}
  {@render crumbs(deployment.name, `Run #${runNumber}`)}
  <div class="page-header">
    <div>
      <h1 class="flex gap-8" style="align-items: center">
        Run #{runNumber} <StatusPill status={entry.status} />
        {#if entry.is_current}<span class="pill pill-gray">current</span>{/if}
      </h1>
      <p>
        {entry.triggered_by}{entry.commit_sha ? ` · ${entry.commit_sha.slice(0, 8)}` : ""} &middot; {fmtWhenFull(entry.started_at)}{duration
          ? ` · ${duration}`
          : ""}
      </p>
    </div>
    {#if entry.status === "success" && !entry.is_current}
      <button class="btn" onclick={rollback} disabled={rollbackBusy}>{rollbackBusy ? "Rolling back..." : "Revert to this"}</button>
    {/if}
  </div>

  {#if error}<div class="error-banner">{error}</div>{/if}

  <div class="card run-step-card">
    <div class="run-step-tag">{deployment.build_strategy === "compose" ? "COMPOSE" : "DEPLOY"}</div>
    <div class="run-step-row">
      {#if succeeded}
        <Icon name="check-circle" class="run-step-icon run-step-icon-success" />
      {:else if failed}
        <Icon name="x-circle" class="run-step-icon run-step-icon-failed" />
      {:else}
        <StatusPill status={entry.status} />
      {/if}
      <span class="run-step-title">{entry.commit_message || entry.git_ref || deployment.name}</span>
      <span class="mono text-dim">{deployment.build_strategy}</span>
      {#if duration}<span class="text-dim run-step-duration">{duration}</span>{/if}
    </div>
    {#if entry.error_message}
      <CodeBlock tone="danger" text={entry.error_message} />
    {:else}
      <p class="text-dim run-step-no-output">
        {succeeded
          ? "No build output is kept for a successful run -- see the Logs tab for live container output."
          : "No output captured for this run."}
      </p>
    {/if}
    {#if entry.rollback_of_deploy_history_id}
      <div class="field-hint">
        Rollback of run #{history.length - history.findIndex((h) => h.id === entry.rollback_of_deploy_history_id)}.
      </div>
    {/if}
  </div>
{/if}
