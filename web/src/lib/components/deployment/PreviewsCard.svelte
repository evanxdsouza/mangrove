<script lang="ts">
  import { api, type Deployment } from "../../api";
  import { errMsg } from "../../errors";
  import StatusPill from "../StatusPill.svelte";

  // On a production deployment's page (same prerequisites as StagingCard):
  // opt into per-PR preview deployments and list the open ones. Previews
  // are created and torn down by the pull_request webhook
  // (handlePullRequestEvent), which also comments each preview's URL on the PR.
  let { projectId, productionDeployment }: { projectId: number; productionDeployment: Deployment } = $props();

  let repo = $state<{ id: number } | null>(null);
  let previews = $state<Deployment[] | null>(null);
  let enabled = $state(false);
  let busy = $state(false);
  let error = $state<string | null>(null);

  $effect(() => {
    const id = productionDeployment.id;
    api
      .get<{ id: number }>(`/api/projects/${projectId}/repo`)
      .then((r) => (repo = r))
      .catch(() => (repo = null));
    api
      .get<Deployment[]>(`/api/deployments/${id}/previews`)
      .then((list) => (previews = list ?? []))
      .catch(() => (previews = []));
  });

  $effect(() => {
    enabled = productionDeployment.pr_previews_enabled;
  });

  async function toggle() {
    const next = !enabled;
    busy = true;
    error = null;
    try {
      await api.post(`/api/deployments/${productionDeployment.id}/pr-previews`, { enabled: next });
      enabled = next;
    } catch (e) {
      error = errMsg(e);
    } finally {
      busy = false;
    }
  }
</script>

{#if repo}
  <div class="card">
    <div class="card-title">PR previews</div>
    <p class="text-dim" style="margin-top: 0">
      When enabled, every open pull request against this repo gets its own preview deployment, deployed on push and torn
      down when the PR closes -- with a bot comment on the PR linking to it.
    </p>
    {#if error}<div class="error-banner">{error}</div>{/if}
    {#if previews && previews.length > 0}
      <div class="kv-list" style="margin-bottom: 14px">
        {#each previews as p (p.id)}
          <a class="kv-row row-link" href="/projects/{projectId}/deployments/{p.id}">
            <span class="kv-key">PR #{p.pr_number} <span class="text-dim mono">({p.git_branch})</span></span>
            <span class="kv-value"><StatusPill status={p.status} /></span>
          </a>
        {/each}
      </div>
    {/if}
    <label class="flex gap-8" style="align-items: center; cursor: pointer">
      <input type="checkbox" checked={enabled} onchange={toggle} disabled={busy} />
      Enable PR previews for this repo
    </label>
  </div>
{/if}
