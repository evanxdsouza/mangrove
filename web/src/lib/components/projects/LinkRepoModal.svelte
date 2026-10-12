<script lang="ts">
  import { api, ApiError } from "../../api";
  import Modal from "../Modal.svelte";

  let { projectId, onClose, onLinked }: { projectId: number; onClose: () => void; onLinked: () => void } = $props();

  let pats = $state<{ id: number; label: string }[]>([]);
  let patId = $state<number | "">("");
  let owner = $state("");
  let name = $state("");
  let branch = $state("main");
  let error = $state<string | null>(null);
  let busy = $state(false);
  let result = $state<{ webhook_path: string; webhook_secret: string } | null>(null);

  api
    .get<{ id: number; label: string }[]>("/api/github/pats")
    .then((p) => (pats = p ?? []))
    .catch(() => (pats = []));

  async function submit(e: SubmitEvent) {
    e.preventDefault();
    if (!patId) {
      error = "Add a GitHub Personal Access Token in Admin first.";
      return;
    }
    busy = true;
    error = null;
    try {
      result = await api.post<{ webhook_path: string; webhook_secret: string }>(`/api/projects/${projectId}/repo`, {
        github_pat_id: patId,
        repo_owner: owner,
        repo_name: name,
        default_branch: branch,
      });
    } catch (err) {
      error = err instanceof ApiError ? err.message : "Something went wrong";
    } finally {
      busy = false;
    }
  }
</script>

{#if result}
  {@const r = result}
  <Modal title="Repo connected" onClose={onLinked}>
    {#snippet children()}
      <p>Paste these into the repo's GitHub settings &rarr; Webhooks &rarr; Add webhook. The secret is shown only once.</p>
      <div class="field">
        <label>
          Payload URL
          <input class="input mono" readonly value="https://<your-host>{r.webhook_path}" onfocus={(e) => e.currentTarget.select()} />
        </label>
      </div>
      <div class="field">
        <label>
          Secret
          <input class="input mono" readonly value={r.webhook_secret} onfocus={(e) => e.currentTarget.select()} />
        </label>
      </div>
      <div class="field-hint">Content type: application/json. Events: just the push event.</div>
      <div class="modal-actions"><button class="btn btn-primary" onclick={onLinked}>Done</button></div>
    {/snippet}
  </Modal>
{:else}
  <Modal title="Connect a GitHub repo" {onClose}>
    {#snippet children(close)}
      {#if error}<div class="error-banner">{error}</div>{/if}
      {#if pats.length === 0}<div class="error-banner">No GitHub tokens configured yet -- add one in Admin first.</div>{/if}
      <form onsubmit={submit}>
        <div class="field">
          <label for="repo-pat">Personal access token</label>
          <select id="repo-pat" class="input" bind:value={patId}>
            <option value="">Select a token...</option>
            {#each pats as p (p.id)}<option value={p.id}>{p.label}</option>{/each}
          </select>
        </div>
        <div class="form-row">
          <div class="field">
            <label for="repo-owner">Owner</label>
            <input id="repo-owner" class="input mono" required bind:value={owner} />
          </div>
          <div class="field">
            <label for="repo-name">Repository</label>
            <input id="repo-name" class="input mono" required bind:value={name} />
          </div>
        </div>
        <div class="field">
          <label for="repo-branch">Default branch</label>
          <input id="repo-branch" class="input mono" bind:value={branch} />
        </div>
        <div class="modal-actions">
          <button type="button" class="btn" onclick={close}>Cancel</button>
          <button type="submit" class="btn btn-primary" disabled={busy}>{busy ? "Connecting..." : "Connect"}</button>
        </div>
      </form>
    {/snippet}
  </Modal>
{/if}
