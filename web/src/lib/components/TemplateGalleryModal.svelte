<script lang="ts">
  import { api, ApiError, type TemplateInstallResult, type TemplateSummary } from "../api";
  import { slugify } from "../slug";
  import Modal from "./Modal.svelte";
  import CenterLoading from "./CenterLoading.svelte";
  import TemplateIcon from "./TemplateIcon.svelte";
  import Icon from "./Icon.svelte";

  // Three steps in one dialog flow: pick a template, fill its prompted
  // config, then see what was created (plus generated credentials, shown
  // exactly once).
  let { projectId, onClose, onInstalled }: { projectId: number; onClose: () => void; onInstalled: () => void } = $props();

  function errMsg(e: unknown): string {
    return e instanceof ApiError ? e.message : "Something went wrong";
  }
  function envFieldKey(slugSuffix: string, key: string): string {
    return `${slugSuffix} ${key}`;
  }

  let templates = $state<TemplateSummary[] | null>(null);
  let error = $state<string | null>(null);
  let selected = $state<TemplateSummary | null>(null);
  let result = $state<TemplateInstallResult | null>(null);

  // Install-form state for the selected template.
  let slug = $state("");
  let envValues = $state<Record<string, string>>({});
  let busy = $state(false);

  api
    .get<TemplateSummary[]>("/api/templates")
    .then((t) => (templates = t))
    .catch((e) => (error = errMsg(e)));

  function select(t: TemplateSummary) {
    selected = t;
    slug = slugify(t.name);
    envValues = {};
    error = null;
  }

  const promptedVars = $derived(
    (selected?.deployments ?? []).flatMap((d) => (d.prompted_env ?? []).map((ev) => ({ slugSuffix: d.slug_suffix, ...ev }))),
  );
  const missingRequired = $derived(promptedVars.some((ev) => ev.required && !envValues[envFieldKey(ev.slugSuffix, ev.key)]));

  async function submit(e: SubmitEvent) {
    e.preventDefault();
    if (!selected || missingRequired) return;
    busy = true;
    error = null;
    try {
      const envOverrides: Record<string, Record<string, string>> = {};
      for (const ev of promptedVars) {
        const value = envValues[envFieldKey(ev.slugSuffix, ev.key)];
        if (!value) continue;
        (envOverrides[ev.slugSuffix] ??= {})[ev.key] = value;
      }
      result = await api.post<TemplateInstallResult>(`/api/projects/${projectId}/templates/${selected.key}/install`, {
        slug,
        env_overrides: envOverrides,
      });
    } catch (err) {
      error = errMsg(err);
    } finally {
      busy = false;
    }
  }

  const credentials = $derived(Object.entries(result?.credentials ?? {}));
</script>

{#if result}
  <Modal title="Deployed" onClose={onInstalled}>
    {#snippet children()}
      <div class="kv-list">
        {#each result!.deployments as d (d.deployment_id)}
          <div class="kv-row">
            <span class="kv-key">{d.name}</span>
            <span class="kv-value">
              <a href="/projects/{projectId}/deployments/{d.deployment_id}">{d.slug}</a>
              {#if d.deploy_error}<span style="color: var(--red)"> -- failed: {d.deploy_error}</span>{/if}
            </span>
          </div>
        {/each}
      </div>

      {#if credentials.length > 0}
        <div class="card-title" style="margin-top: 18px">Generated credentials</div>
        <p class="field-hint" style="margin-top: 0">
          Shown only once -- copy these now. They're stored encrypted and can't be displayed again.
        </p>
        {#each credentials as [label, value] (label)}
          <div class="field">
            <label>
              {label}
              <input class="input mono" readonly {value} onfocus={(e) => e.currentTarget.select()} />
            </label>
          </div>
        {/each}
      {/if}

      <div class="modal-actions">
        <button class="btn btn-primary" onclick={onInstalled}>Done</button>
      </div>
    {/snippet}
  </Modal>
{:else if selected}
  {@const template = selected}
  <Modal title="Deploy {template.name}" {onClose}>
    {#snippet children()}
      {#if error}<div class="error-banner">{error}</div>{/if}
      <p class="text-dim" style="margin-top: 0">{template.description}</p>
      <form onsubmit={submit}>
        <div class="field">
          <label for="template-slug">Slug</label>
          <input
            id="template-slug"
            class="input mono"
            required
            value={slug}
            oninput={(e) => (slug = slugify(e.currentTarget.value))}
          />
          <div class="field-hint">
            {template.deployments.length > 1
              ? `Used as a base for ${template.deployments.length} linked deployments (e.g. "${slug}" and "${slug}${template.deployments[0].slug_suffix}").`
              : "The deployment's slug within this project."}
          </div>
        </div>

        {#if promptedVars.length > 0}
          <div class="card-title" style="margin-top: 18px">Configuration</div>
          {#each promptedVars as ev (envFieldKey(ev.slugSuffix, ev.key))}
            {@const fieldKey = envFieldKey(ev.slugSuffix, ev.key)}
            <div class="field">
              <label for="template-env-{fieldKey}">{ev.label || ev.key}{ev.required ? " *" : ""}</label>
              <input
                id="template-env-{fieldKey}"
                class="input mono"
                required={ev.required}
                value={envValues[fieldKey] ?? ""}
                oninput={(e) => (envValues = { ...envValues, [fieldKey]: e.currentTarget.value })}
              />
              <div class="field-hint">{ev.key}</div>
            </div>
          {/each}
        {/if}

        <div class="card-title" style="margin-top: 18px">What this creates</div>
        <div class="kv-list">
          {#each template.deployments as d (d.slug_suffix)}
            <div class="kv-row">
              <span class="kv-key">{slug}{d.slug_suffix}</span>
              <span class="kv-value">
                {d.build_strategy === "dockerfile" ? `built from ${d.git_url}` : d.image_ref} &middot; {d.cpu_limit_cores} CPU / {d.memory_limit_mb}MB{d.force_internal_only ? " · internal only" : ""}
              </span>
            </div>
          {/each}
        </div>
        <div class="field-hint" style="margin-top: 10px">
          Every deployment goes through the same memory/port admission control as a hand-created one -- this needs
          ~{template.total_memory_mb}MB of budget in total. Check Admin &rarr; Resource budget if you're unsure how much is free.
        </div>

        <div class="modal-actions">
          <button type="button" class="btn" onclick={() => (selected = null)} disabled={busy}>Back</button>
          <button type="submit" class="btn btn-primary" disabled={busy || missingRequired}>
            <Icon name="deploy" />
            {busy ? "Deploying..." : "Deploy"}
          </button>
        </div>
      </form>
    {/snippet}
  </Modal>
{:else}
  <Modal title="Deploy from template" {onClose} size="lg">
    {#snippet children()}
      {#if error}<div class="error-banner">{error}</div>{/if}
      {#if templates === null}
        <CenterLoading />
      {:else}
        <div class="template-grid">
          {#each templates as t (t.key)}
            <button type="button" class="card card-clickable template-card" onclick={() => select(t)} style="text-align: left; font: inherit; color: inherit">
              <div class="template-card-top">
                <TemplateIcon templateKey={t.key} category={t.category} class="template-card-icon" />
                <div class="card-title" style="margin: 0">{t.name}</div>
              </div>
              <p class="text-dim template-card-description">{t.description}</p>
              <div class="text-dim" style="font-size: 12px">
                {t.category} &middot; ~{t.total_memory_mb}MB{t.deployments.length > 1 ? ` across ${t.deployments.length} deployments` : ""}
              </div>
            </button>
          {/each}
        </div>
      {/if}
    {/snippet}
  </Modal>
{/if}
