<script lang="ts">
  import { api, type Deployment } from "../../api";
  import { errMsg } from "../../errors";
  import StatusPill from "../StatusPill.svelte";

  // On a production deployment's page (with a linked repo): lists staging
  // deployments created from this one, and spins up a new one tracking an
  // arbitrary branch. See createStagingDeployment.
  let { projectId, productionDeployment }: { projectId: number; productionDeployment: Deployment } = $props();

  let repo = $state<{ id: number } | null>(null);
  let staging = $state<Deployment[] | null>(null);
  let branch = $state("staging");
  let busy = $state(false);
  let error = $state<string | null>(null);

  function loadStaging(id: number) {
    api
      .get<Deployment[]>(`/api/deployments/${id}/staging`)
      .then((list) => (staging = list ?? []))
      .catch(() => (staging = []));
  }

  $effect(() => {
    const id = productionDeployment.id;
    api
      .get<{ id: number }>(`/api/projects/${projectId}/repo`)
      .then((r) => (repo = r))
      .catch(() => (repo = null));
    loadStaging(id);
  });

  async function create() {
    busy = true;
    error = null;
    try {
      await api.post(`/api/deployments/${productionDeployment.id}/staging`, { branch });
      loadStaging(productionDeployment.id);
    } catch (e) {
      error = errMsg(e);
    } finally {
      busy = false;
    }
  }
</script>

{#if repo}
  <div class="card">
    <div class="card-title">Staging environments</div>
    <p class="text-dim" style="margin-top: 0">
      A staging environment tracks its own branch, auto-deploys independently, and can be promoted into this deployment once
      verified.
    </p>
    {#if error}<div class="error-banner">{error}</div>{/if}
    {#if staging && staging.length > 0}
      <div class="kv-list" style="margin-bottom: 14px">
        {#each staging as s (s.id)}
          <a class="kv-row row-link" href="/projects/{projectId}/deployments/{s.id}">
            <span class="kv-key">{s.name} <span class="text-dim mono">({s.git_branch})</span></span>
            <span class="kv-value"><StatusPill status={s.status} /></span>
          </a>
        {/each}
      </div>
    {/if}
    <div class="form-row" style="align-items: flex-end">
      <div class="field" style="margin-bottom: 0">
        <label for="staging-branch">Branch to stage</label>
        <input id="staging-branch" class="input mono" bind:value={branch} />
      </div>
      <button class="btn btn-sm btn-primary" onclick={create} disabled={busy || !branch}>{busy ? "Creating..." : "+ New staging environment"}</button>
    </div>
  </div>
{/if}
