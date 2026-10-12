<script lang="ts">
  import { api, ApiError, type DetectedEnvVar, type DetectionResult, type GithubRepo, type Project } from "../../api";
  import { slugify } from "../../slug";

  type Strategy = "dockerfile" | "nixpacks" | "compose" | "static";

  let {
    projectId,
    credId,
    repo,
    branch = $bindable(),
    onBack,
    onDeployed,
  }: {
    projectId?: number;
    credId?: number;
    repo: GithubRepo;
    branch: string;
    onBack: () => void;
    onDeployed: (projectId: number, deploymentId: number) => void;
  } = $props();

  function errMsg(e: unknown): string {
    return e instanceof ApiError ? e.message : "Something went wrong";
  }

  let detecting = $state(true);
  let detectError = $state<string | null>(null);
  let strategy = $state<Strategy>("dockerfile");
  let dockerfilePath = $state("Dockerfile");
  let composePath = $state("docker-compose.yml");
  let staticBuildCommand = $state("");
  // Empty, not "dist": detection leaves this blank for a repo it found no
  // build step for (index.html at the root, no package.json) -- there,
  // "dist" would silently point the static copy at a subdirectory that
  // doesn't exist instead of the repo root.
  let staticOutputDir = $state("");
  let envVars = $state<DetectedEnvVar[]>([]);
  let envValues = $state<Record<string, string>>({});

  // svelte-ignore state_referenced_locally
  let appName = $state(repo.name);
  // svelte-ignore state_referenced_locally
  let slug = $state(slugify(repo.name));
  let serviceName = $state("web");
  let internalPort = $state(3000);
  let internalOnly = $state(true);
  let memoryMB = $state(256);
  let cpuCores = $state(0.5);

  let busy = $state(false);
  let error = $state<string | null>(null);
  let result = $state<{ projectId: number; deploymentId: number; deployError?: string } | null>(null);

  $effect(() => {
    const body = { github_pat_id: credId, repo_owner: repo.owner, repo_name: repo.name, branch, root_path: "." };
    detecting = true;
    detectError = null;
    api
      .post<DetectionResult>("/api/github/detect", body)
      .then((r) => {
        strategy = r.build_strategy as Strategy;
        if (r.dockerfile_path) dockerfilePath = r.dockerfile_path;
        if (r.compose_path) composePath = r.compose_path;
        if (r.suggested_port) internalPort = r.suggested_port;
        staticBuildCommand = r.static_build_command ?? "";
        staticOutputDir = r.static_output_dir ?? "";
        envVars = r.env_vars ?? [];
      })
      .catch((e) => (detectError = errMsg(e)))
      .finally(() => (detecting = false));
  });

  async function submit() {
    busy = true;
    error = null;
    try {
      let pid = projectId;
      if (!pid) {
        const project = await api.post<Project>("/api/projects", { name: appName, slug, description: "" });
        pid = project.id;
      }

      const repoLink = await api.post<{ id: number }>(`/api/projects/${pid}/repo`, {
        github_pat_id: credId,
        repo_owner: repo.owner,
        repo_name: repo.name,
        default_branch: branch,
      });

      const payload: Record<string, unknown> = {
        name: appName,
        slug,
        build_strategy: strategy,
        git_branch: branch,
        root_path: ".",
        dockerfile_path: dockerfilePath,
        compose_path: composePath,
        static_build_command: staticBuildCommand,
        static_output_dir: staticOutputDir,
        image_retention_count: 5,
      };
      if (strategy === "static") {
        payload.service = { name: serviceName, is_internal_only: internalOnly };
      } else if (strategy !== "compose") {
        payload.service = {
          name: serviceName,
          internal_port: internalPort,
          is_internal_only: internalOnly,
          cpu_limit_cores: cpuCores,
          memory_limit_mb: memoryMB,
          health_check_path: "/",
        };
      }
      const dep = await api.post<{ id: number }>(`/api/projects/${pid}/deployments`, payload);

      await api.post(`/api/deployments/${dep.id}/repo`, { project_repo_id: repoLink.id, branch, auto_deploy_on_push: true });

      if (strategy !== "compose" && envVars.length > 0) {
        const services = await api.get<{ id: number }[]>(`/api/deployments/${dep.id}/services`);
        const serviceId = services?.[0]?.id;
        if (serviceId) {
          for (const ev of envVars) {
            const value = envValues[ev.key];
            if (!value) continue;
            await api.put(`/api/services/${serviceId}/env/${ev.key}`, { value, is_secret: ev.secret });
          }
        }
      }

      let deployError: string | undefined;
      try {
        await api.post(`/api/deployments/${dep.id}/redeploy`, {});
      } catch (e) {
        deployError = errMsg(e);
      }
      result = { projectId: pid, deploymentId: dep.id, deployError };
    } catch (e) {
      error = errMsg(e);
    } finally {
      busy = false;
    }
  }
</script>

{#if result}
  {@const r = result}
  <p><strong>{repo.owner}/{repo.name}</strong> is set up and deploying.</p>
  {#if r.deployError}<div class="error-banner">The first deploy failed to start: {r.deployError}</div>{/if}
  <div class="modal-actions">
    <button class="btn btn-primary" onclick={() => onDeployed(r.projectId, r.deploymentId)}>View deployment</button>
  </div>
{:else}
  {#if error}<div class="error-banner">{error}</div>{/if}
  {#if detectError}
    <div class="error-banner">Couldn't auto-detect this repo ({detectError}) -- review the settings below manually.</div>
  {/if}

  {#if !projectId}
    <div class="form-row">
      <div class="field">
        <label for="ghw-name">Name</label>
        <input
          id="ghw-name"
          class="input"
          required
          value={appName}
          oninput={(e) => {
            appName = e.currentTarget.value;
            slug = slugify(appName);
          }}
        />
      </div>
      <div class="field">
        <label for="ghw-slug">Slug</label>
        <input id="ghw-slug" class="input mono" required value={slug} oninput={(e) => (slug = slugify(e.currentTarget.value))} />
      </div>
    </div>
  {/if}

  <div class="field">
    <label for="ghw-branch">Branch</label>
    <input id="ghw-branch" class="input mono" bind:value={branch} />
  </div>

  <div class="field">
    <label for="ghw-strategy">Build strategy {#if detecting}<span class="text-faint">(detecting...)</span>{/if}</label>
    <select id="ghw-strategy" class="input" bind:value={strategy} disabled={detecting}>
      <option value="dockerfile">Dockerfile</option>
      <option value="nixpacks">Nixpacks (no Dockerfile needed)</option>
      <option value="compose">Docker Compose (multi-service)</option>
      <option value="static">Static site (build + serve files, no container)</option>
    </select>
  </div>

  {#if strategy === "dockerfile"}
    <div class="field">
      <label for="ghw-dockerfile">Dockerfile path</label>
      <input id="ghw-dockerfile" class="input mono" bind:value={dockerfilePath} />
    </div>
  {:else if strategy === "compose"}
    <div class="field">
      <label for="ghw-compose">Compose file path</label>
      <input id="ghw-compose" class="input mono" bind:value={composePath} />
    </div>
    <div class="field-hint">Compose stacks have per-service env vars -- set them from each service's page after the first deploy.</div>
  {:else if strategy === "static"}
    <div class="field">
      <label for="ghw-build-cmd">Build command (optional)</label>
      <input id="ghw-build-cmd" class="input mono" placeholder="npm run build -- leave blank if already pre-built" bind:value={staticBuildCommand} />
    </div>
    <div class="field">
      <label for="ghw-output-dir">Output directory</label>
      <input id="ghw-output-dir" class="input mono" placeholder="dist -- leave blank to use the repo root as-is" bind:value={staticOutputDir} />
    </div>
  {/if}

  {#if strategy !== "compose"}
    <div class="card-title" style="margin-top: 18px">Service</div>
    <div class="field">
      <label for="ghw-service-name">Service name</label>
      <input id="ghw-service-name" class="input mono" bind:value={serviceName} />
    </div>
    {#if strategy !== "static"}
      <div class="form-row">
        <div class="field">
          <label for="ghw-port">Internal port</label>
          <input id="ghw-port" class="input" type="number" bind:value={internalPort} />
        </div>
        <div class="field">
          <label for="ghw-memory">Memory limit (MB)</label>
          <input id="ghw-memory" class="input" type="number" bind:value={memoryMB} />
        </div>
        <div class="field">
          <label for="ghw-cpu">CPU cores</label>
          <input id="ghw-cpu" class="input" type="number" step="0.1" bind:value={cpuCores} />
        </div>
      </div>
    {/if}
    <div class="field">
      <label style="display: flex; align-items: center; gap: 8px">
        <input type="checkbox" bind:checked={internalOnly} />
        Internal only (never expose a public port)
      </label>
    </div>
  {/if}

  {#if strategy !== "compose" && envVars.length > 0}
    <div class="card-title" style="margin-top: 18px">Environment variables found in this repo</div>
    {#each envVars as ev (ev.key)}
      <div class="field">
        <label for="ghw-env-{ev.key}">{ev.key}{#if ev.secret}<span class="text-faint"> (secret)</span>{/if}</label>
        <input
          id="ghw-env-{ev.key}"
          class="input mono"
          type={ev.secret ? "password" : "text"}
          value={envValues[ev.key] ?? ""}
          oninput={(e) => (envValues = { ...envValues, [ev.key]: e.currentTarget.value })}
        />
      </div>
    {/each}
  {/if}

  <div class="modal-actions">
    <button type="button" class="btn" onclick={onBack} disabled={busy}>Back</button>
    <button type="button" class="btn btn-primary" onclick={submit} disabled={busy || detecting || !slug}>
      {busy ? "Deploying..." : "Deploy"}
    </button>
  </div>
{/if}
