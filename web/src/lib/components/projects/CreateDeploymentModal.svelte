<script lang="ts">
  import { api, ApiError, type Deployment } from "../../api";
  import { slugify } from "../../slug";
  import Modal from "../Modal.svelte";

  let { projectId, onClose, onCreated }: { projectId: number; onClose: () => void; onCreated: (id: number) => void } = $props();

  type Strategy = "dockerfile" | "nixpacks" | "compose" | "image" | "static";

  let name = $state("");
  let slug = $state("");
  let strategy = $state<Strategy>("dockerfile");
  let gitBranch = $state("main");
  let imageRef = $state("");
  let dockerfilePath = $state("");
  let composePath = $state("docker-compose.yml");
  let staticBuildCommand = $state("");
  let staticOutputDir = $state("dist");
  let rootPath = $state(".");
  let serviceName = $state("web");
  let internalPort = $state(3000);
  // Internal-only is the explicit default -- exposing a public port is
  // something the user opts into, not an accidental default.
  let internalOnly = $state(true);
  let memoryMB = $state(256);
  let cpuCores = $state(0.5);
  let replicas = $state(1);
  let healthPath = $state("/");
  let error = $state<string | null>(null);
  let busy = $state(false);

  async function submit(e: SubmitEvent) {
    e.preventDefault();
    busy = true;
    error = null;
    try {
      const payload: Record<string, unknown> = {
        name,
        slug,
        build_strategy: strategy,
        git_branch: gitBranch,
        image_ref: imageRef,
        root_path: rootPath,
        dockerfile_path: dockerfilePath,
        compose_path: composePath,
        static_build_command: staticBuildCommand,
        static_output_dir: staticOutputDir,
        image_retention_count: 5,
        replicas: strategy === "compose" || strategy === "static" ? 1 : replicas,
      };
      if (strategy === "static") {
        // No container ever runs for a static site -- just the service name
        // (required by the backend) and whether it gets a public port.
        payload.service = { name: serviceName, is_internal_only: internalOnly };
      } else if (strategy !== "compose") {
        payload.service = {
          name: serviceName,
          internal_port: internalPort,
          is_internal_only: internalOnly,
          cpu_limit_cores: cpuCores,
          memory_limit_mb: memoryMB,
          health_check_path: healthPath,
        };
      }
      const dep = await api.post<Deployment>(`/api/projects/${projectId}/deployments`, payload);
      onCreated(dep.id);
    } catch (err) {
      error = err instanceof ApiError ? err.message : "Something went wrong";
    } finally {
      busy = false;
    }
  }
</script>

<Modal title="New deployment" {onClose}>
  {#snippet children(close)}
    {#if error}<div class="error-banner">{error}</div>{/if}
    <form onsubmit={submit}>
      <div class="form-row">
        <div class="field">
          <label for="dep-name">Name</label>
          <input
            id="dep-name"
            class="input"
            required
            value={name}
            oninput={(e) => {
              name = e.currentTarget.value;
              if (!slug) slug = slugify(name);
            }}
          />
        </div>
        <div class="field">
          <label for="dep-slug">Slug</label>
          <input id="dep-slug" class="input mono" required value={slug} oninput={(e) => (slug = slugify(e.currentTarget.value))} />
        </div>
      </div>

      <div class="field">
        <label for="dep-strategy">Build strategy</label>
        <select id="dep-strategy" class="input" bind:value={strategy}>
          <option value="dockerfile">Dockerfile</option>
          <option value="nixpacks">Nixpacks (no Dockerfile needed)</option>
          <option value="compose">Docker Compose (multi-service)</option>
          <option value="image">Existing image</option>
          <option value="static">Static site (build + serve files, no container)</option>
        </select>
      </div>

      {#if strategy === "image"}
        <div class="field">
          <label for="dep-image-ref">Image reference</label>
          <input id="dep-image-ref" class="input mono" placeholder="nginx:alpine" bind:value={imageRef} />
        </div>
      {:else}
        <div class="form-row">
          <div class="field">
            <label for="dep-branch">Branch</label>
            <input id="dep-branch" class="input mono" bind:value={gitBranch} />
          </div>
          <div class="field">
            <label for="dep-root-path">Root path</label>
            <input id="dep-root-path" class="input mono" bind:value={rootPath} />
          </div>
        </div>
        {#if strategy === "dockerfile"}
          <div class="field">
            <label for="dep-dockerfile-path">Dockerfile path (optional)</label>
            <input id="dep-dockerfile-path" class="input mono" placeholder="Dockerfile" bind:value={dockerfilePath} />
          </div>
        {:else if strategy === "compose"}
          <div class="field">
            <label for="dep-compose-path">Compose file path</label>
            <input id="dep-compose-path" class="input mono" bind:value={composePath} />
          </div>
        {:else if strategy === "static"}
          <div class="field">
            <label for="dep-static-build-command">Build command (optional)</label>
            <input
              id="dep-static-build-command"
              class="input mono"
              placeholder="npm run build -- leave blank if the repo is already pre-built HTML/CSS/JS"
              bind:value={staticBuildCommand}
            />
          </div>
          <div class="field">
            <label for="dep-static-output-dir">Output directory</label>
            <input id="dep-static-output-dir" class="input mono" placeholder="dist" bind:value={staticOutputDir} />
            <div class="field-hint">Relative to the build's working directory (or to root path above, if there's no build command).</div>
          </div>
        {/if}
      {/if}

      {#if strategy !== "compose"}
        <div class="card-title" style="margin-top: 18px">Service</div>
        <div class="field">
          <label for="dep-service-name">Service name</label>
          <input id="dep-service-name" class="input mono" bind:value={serviceName} />
        </div>
        {#if strategy !== "static"}
          <div class="form-row">
            <div class="field">
              <label for="dep-internal-port">Internal port</label>
              <input id="dep-internal-port" class="input" type="number" bind:value={internalPort} />
            </div>
            <div class="field">
              <label for="dep-memory">Memory limit (MB)</label>
              <input id="dep-memory" class="input" type="number" bind:value={memoryMB} />
            </div>
          </div>
          <div class="form-row">
            <div class="field">
              <label for="dep-cpu">CPU cores</label>
              <input id="dep-cpu" class="input" type="number" step="0.1" bind:value={cpuCores} />
            </div>
          </div>
          <div class="field">
            <label for="dep-health-path">Health check path (optional)</label>
            <input id="dep-health-path" class="input mono" bind:value={healthPath} />
          </div>
          <div class="field">
            <label for="dep-replicas">Replicas</label>
            <input
              id="dep-replicas"
              class="input"
              type="number"
              min="1"
              max="32"
              value={replicas}
              oninput={(e) => (replicas = Math.max(1, Number(e.currentTarget.value) || 1))}
            />
            <div class="field-hint">How many containers of this image to run behind one load-balanced route.</div>
          </div>
        {/if}
        <div class="field">
          <label style="display: flex; align-items: center; gap: 8px">
            <input type="checkbox" bind:checked={internalOnly} />
            Internal only (never expose a public port)
          </label>
          {#if strategy === "static"}
            <div class="field-hint">A static site has no container to run or health-check -- Caddy serves the built files directly once this is off.</div>
          {/if}
          <div class="field-hint">This is the explicit default choice Mangrove asks for at deploy time -- flip it off to expose the app publicly.</div>
        </div>
      {/if}

      <div class="modal-actions">
        <button type="button" class="btn" onclick={close}>Cancel</button>
        <button type="submit" class="btn btn-primary" disabled={busy}>{busy ? "Creating..." : "Create deployment"}</button>
      </div>
    </form>
  {/snippet}
</Modal>
