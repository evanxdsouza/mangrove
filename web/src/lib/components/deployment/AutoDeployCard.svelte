<script lang="ts">
  import { api, type Deployment } from "../../api";
  import WebhookHealth from "./WebhookHealth.svelte";

  let { projectId, deploymentId, deployment, onSaved }: { projectId: number; deploymentId: number; deployment: Deployment | null; onSaved: () => void } =
    $props();

  interface ProjectRepoInfo {
    id: number;
    repo_owner: string;
    repo_name: string;
    webhook_registered: boolean;
  }

  let repo = $state<ProjectRepoInfo | null>(null);
  let branch = $state("");
  let autoDeploy = $state(false);
  let busy = $state(false);
  let saved = $state(false);

  $effect(() => {
    api
      .get<ProjectRepoInfo>(`/api/projects/${projectId}/repo`)
      .then((r) => (repo = r))
      .catch(() => (repo = null));
  });

  let synced = "";
  $effect(() => {
    if (!deployment) return;
    const sig = JSON.stringify([deployment.git_branch, deployment.auto_deploy_on_push]);
    if (sig === synced) return;
    synced = sig;
    branch = deployment.git_branch ?? "main";
    autoDeploy = deployment.auto_deploy_on_push;
  });

  async function save() {
    if (!repo) return;
    busy = true;
    saved = false;
    try {
      await api.post(`/api/deployments/${deploymentId}/repo`, { project_repo_id: repo.id, branch, auto_deploy_on_push: autoDeploy });
      saved = true;
      onSaved();
    } finally {
      busy = false;
    }
  }
</script>

<!-- No repo connected to this project: nothing to configure. -->
{#if repo}
  <div class="card">
    <div class="card-title">Auto-deploy</div>
    <p class="text-dim" style="margin-top: 0">Deploy automatically when {repo.repo_owner}/{repo.repo_name} receives a push to the branch below.</p>
    <div class="form-row" style="align-items: flex-end">
      <div class="field" style="margin-bottom: 0">
        <label for="autodeploy-branch">Branch</label>
        <input id="autodeploy-branch" class="input mono" bind:value={branch} />
      </div>
      <div class="field" style="margin-bottom: 0; flex: 0 0 auto">
        <label style="display: flex; align-items: center; gap: 6px; white-space: nowrap">
          <input type="checkbox" bind:checked={autoDeploy} />
          Auto-deploy on push
        </label>
      </div>
      <button class="btn btn-sm" onclick={save} disabled={busy}>{busy ? "Saving..." : "Save"}</button>
    </div>
    {#if saved}<div class="field-hint">Saved.</div>{/if}
    <WebhookHealth {projectId} {repo} />
  </div>
{/if}
