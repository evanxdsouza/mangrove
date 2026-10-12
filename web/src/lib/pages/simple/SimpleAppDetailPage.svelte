<script lang="ts">
  import { goto } from "$app/navigation";
  import { api, type Deployment } from "../../api";
  import { errMsg } from "../../errors";
  import { setMode } from "../../stores/ui-mode.svelte";
  import { workspaceRole } from "../../stores/workspaces.svelte";
  import Icon from "../../components/Icon.svelte";
  import ConfirmModal from "../../components/ConfirmModal.svelte";
  import PlainStatusPill from "./PlainStatusPill.svelte";

  // The simple-mode counterpart to the deployment page -- no env vars, no
  // CPU/memory numbers, no build internals. Just status in plain words, and
  // the two actions a non-technical user needs: try again if it's broken,
  // remove it when they're done with it.
  let { deploymentId }: { deploymentId: number } = $props();

  let deployment = $state<Deployment | null>(null);
  let projectWorkspaceId = $state<number | null>(null);
  let error = $state<string | null>(null);
  let retrying = $state(false);
  let showDelete = $state(false);
  const isAdmin = $derived(workspaceRole(projectWorkspaceId) === "admin");

  function load() {
    api
      .get<Deployment>(`/api/deployments/${deploymentId}`)
      .then((d) => {
        deployment = d;
        error = null;
      })
      .catch((e) => (error = errMsg(e)));
  }

  $effect(() => {
    void deploymentId;
    load();
    const interval = setInterval(load, 4000);
    return () => clearInterval(interval);
  });

  // A deployment doesn't carry workspace_id directly -- one lookup per project.
  const projectId = $derived(deployment?.project_id);
  $effect(() => {
    if (projectId == null) return;
    api
      .get<{ workspace_id: number }>(`/api/projects/${projectId}`)
      .then((p) => (projectWorkspaceId = p.workspace_id))
      .catch(() => {});
  });

  async function retry() {
    retrying = true;
    error = null;
    try {
      await api.post(`/api/deployments/${deploymentId}/deploy`, {});
      load();
    } catch (e) {
      error = errMsg(e);
    } finally {
      retrying = false;
    }
  }

  const STATUS_TEXT: Record<string, string> = {
    running: "This app is up and running.",
    failed: "This app is having trouble starting. Try again below, or switch to the advanced view for more detail.",
    pending: "This app is getting set up. This can take a minute.",
    building: "This app is getting set up. This can take a minute.",
    stopped: "This app isn't running right now.",
    sleeping: "This app went to sleep after a quiet period. It'll wake up automatically the next time someone visits.",
  };
</script>

<nav class="breadcrumb" aria-label="Breadcrumb">
  <a href="/" class="breadcrumb-back"><Icon name="chevron-down" /> Your apps</a>
  <span>/</span>
  <span>{deployment?.name ?? "..."}</span>
</nav>
<div class="page-header">
  <div>
    <h1>{deployment?.name ?? "Loading..."}</h1>
    {#if deployment}
      <p class="flex gap-8" style="align-items: center"><PlainStatusPill status={deployment.status} /></p>
    {/if}
  </div>
  <div class="flex gap-8">
    {#if deployment?.status === "failed"}
      <button class="btn btn-primary" onclick={retry} disabled={retrying}><Icon name="deploy" /> {retrying ? "Trying again..." : "Try again"}</button>
    {/if}
    {#if isAdmin}
      <button class="btn btn-danger" onclick={() => (showDelete = true)}><Icon name="trash" /> Remove app</button>
    {/if}
  </div>
</div>

{#if error}<div class="error-banner">{error}</div>{/if}

<div class="card">
  <div class="card-title">Status</div>
  <p style="margin: 0">{deployment ? (STATUS_TEXT[deployment.status] ?? "") : ""}</p>
</div>

<button type="button" class="btn btn-sm" onclick={() => setMode("technical")}><Icon name="gear" /> Switch to advanced view</button>

{#if showDelete}
  <ConfirmModal
    title="Remove app"
    body={`This removes "${deployment?.name ?? "this app"}" and everything it stored. This can't be undone.`}
    confirmLabel="Remove app"
    onClose={() => (showDelete = false)}
    onConfirm={async () => {
      await api.del(`/api/deployments/${deploymentId}`);
      goto("/");
    }}
  />
{/if}
