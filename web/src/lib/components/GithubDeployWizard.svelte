<script lang="ts">
  import { api, ApiError, type GithubCredential, type GithubRepo } from "../api";
  import Modal from "./Modal.svelte";
  import CenterLoading from "./CenterLoading.svelte";
  import Icon from "./Icon.svelte";
  import GithubConfigureStep from "./github/GithubConfigureStep.svelte";

  // "Deploy from GitHub": connect (OAuth popup) -> pick a repo (private
  // repos included, via the connected token) -> auto-detect a build
  // strategy and prompt for the env vars the repo declares -> deploy. Every
  // step after OAuth composes calls the technical flow already exposes
  // (link repo, create deployment, set env vars, redeploy).
  //
  // projectId is omitted when opened from Simple mode -- then a project is
  // created behind the scenes, named after the chosen repo.
  let {
    projectId,
    onClose,
    onDeployed,
  }: { projectId?: number; onClose: () => void; onDeployed: (projectId: number, deploymentId: number) => void } = $props();

  type Step = "connect" | "repo" | "configure";

  function errMsg(e: unknown): string {
    return e instanceof ApiError ? e.message : "Something went wrong";
  }

  let step = $state<Step>("connect");
  let creds = $state<GithubCredential[] | null>(null);
  let credId = $state<number | "">("");
  let error = $state<string | null>(null);
  let repos = $state<GithubRepo[] | null>(null);
  let filter = $state("");
  let selectedRepo = $state<GithubRepo | null>(null);
  let branch = $state("main");

  function loadCreds() {
    api
      .get<GithubCredential[]>("/api/github/pats")
      .then((list) => {
        const all = list ?? [];
        creds = all;
        if (all.length > 0) {
          if (credId === "") credId = all[0].id;
          if (step === "connect") step = "repo";
        }
      })
      .catch((e) => (error = errMsg(e)));
  }
  loadCreds();

  $effect(() => {
    if (step !== "repo" || !credId) return;
    const id = credId;
    repos = null;
    api
      .get<GithubRepo[]>(`/api/github/repos?github_pat_id=${id}`)
      .then((r) => (repos = r ?? []))
      .catch((e) => (error = errMsg(e)));
  });

  function connect() {
    error = null;
    const popup = window.open("/api/github/oauth/start", "mangrove-github-oauth", "width=680,height=780");
    if (!popup) {
      error = "Your browser blocked the popup -- allow popups for this site and try again.";
      return;
    }
    const handler = (ev: MessageEvent) => {
      if (ev.origin !== window.location.origin) return;
      const data = ev.data as { type?: string; ok?: boolean; error?: string } | undefined;
      if (!data || data.type !== "mangrove-github-oauth") return;
      window.removeEventListener("message", handler);
      if (data.ok) loadCreds();
      else error = data.error || "GitHub connection failed.";
    };
    window.addEventListener("message", handler);
  }

  const filteredRepos = $derived.by(() => {
    const list = repos ?? [];
    if (!filter.trim()) return list;
    const needle = filter.toLowerCase();
    return list.filter((r) => `${r.owner}/${r.name}`.toLowerCase().includes(needle));
  });

  function pickRepo(r: GithubRepo) {
    selectedRepo = r;
    branch = r.default_branch || "main";
    step = "configure";
  }
</script>

<Modal title="Deploy from GitHub" {onClose} size="lg">
  {#snippet children(close)}
    {#if error}<div class="error-banner">{error}</div>{/if}

    {#if step === "connect"}
      <p class="text-dim" style="margin-top: 0">
        Connect a GitHub account to pick a repo (including private ones) without pasting a token by hand.
      </p>
      <div class="modal-actions" style="justify-content: flex-start">
        <button type="button" class="btn btn-primary" onclick={connect}><Icon name="branch" /> Connect GitHub</button>
      </div>
      {#if creds && creds.length === 0}
        <p class="field-hint">
          You can also use a pasted Personal Access Token from Admin -- if you add one there, come back and reopen this dialog.
        </p>
      {/if}
      <div class="modal-actions">
        <button type="button" class="btn" onclick={close}>Cancel</button>
      </div>
    {:else if step === "repo"}
      {#if creds && creds.length > 1}
        <div class="field">
          <label for="gh-cred">GitHub account</label>
          <select id="gh-cred" class="input" bind:value={credId}>
            {#each creds as c (c.id)}
              <option value={c.id}>{c.source === "oauth" ? `@${c.github_login}` : c.label}</option>
            {/each}
          </select>
        </div>
      {/if}
      <div class="field">
        <label for="gh-filter">Find a repo</label>
        <!-- svelte-ignore a11y_autofocus -->
        <input id="gh-filter" class="input" placeholder="owner/name" bind:value={filter} autofocus />
      </div>
      {#if repos === null}
        <CenterLoading />
      {:else if filteredRepos.length === 0}
        <div class="card empty-state"><p>No matching repos.</p></div>
      {:else}
        <div class="kv-list" style="max-height: 320px; overflow-y: auto">
          {#each filteredRepos as r (`${r.owner}/${r.name}`)}
            <button type="button" class="kv-row row-link" onclick={() => pickRepo(r)} style="cursor: pointer; background: none; border: none; border-bottom: 1px solid var(--border-soft); text-align: left; width: 100%">
              <span class="kv-key mono">{r.owner}/{r.name}{#if r.private}<span class="text-faint"> (private)</span>{/if}</span>
              <span class="kv-value text-dim" style="font-size: 12px">{r.description || ""}</span>
            </button>
          {/each}
        </div>
      {/if}
      <div class="modal-actions">
        <button type="button" class="btn" onclick={close}>Cancel</button>
        <button type="button" class="btn" onclick={connect}>Connect another account</button>
      </div>
    {:else if step === "configure" && selectedRepo}
      <GithubConfigureStep
        {projectId}
        credId={credId === "" ? undefined : credId}
        repo={selectedRepo}
        bind:branch
        onBack={() => (step = "repo")}
        {onDeployed}
      />
    {/if}
  {/snippet}
</Modal>
