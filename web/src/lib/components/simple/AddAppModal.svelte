<script lang="ts">
  import { api, ApiError, type Project, type TemplateInstallResult, type TemplateSummary } from "../../api";
  import { SIMPLE_TEMPLATES } from "../../plainCopy";
  import { slugify } from "../../slug";
  import Modal from "../Modal.svelte";
  import CenterLoading from "../CenterLoading.svelte";
  import TemplateIcon from "../TemplateIcon.svelte";

  // Simple mode's "add an app" flow collapses the technical two-step flow
  // (create a project, then install a template into it) into one click: it
  // creates a project behind the scenes, named after the app, and installs
  // the chosen template straight into it. The project concept never
  // surfaces in this mode's UI.
  let { onClose, onAdded }: { onClose: () => void; onAdded: (projectId: number, deploymentId: number) => void } = $props();

  function errMsg(e: unknown): string {
    return e instanceof ApiError ? e.message : "Something went wrong";
  }

  let templates = $state<TemplateSummary[] | null>(null);
  let error = $state<string | null>(null);
  let installing = $state<string | null>(null);
  let result = $state<{ templateKey: string; res: TemplateInstallResult; projectId: number } | null>(null);

  api
    .get<TemplateSummary[]>("/api/templates")
    .then((all) => (templates = (all ?? []).filter((t) => t.key in SIMPLE_TEMPLATES)))
    .catch((e) => (error = errMsg(e)));

  async function install(template: TemplateSummary) {
    const copy = SIMPLE_TEMPLATES[template.key];
    installing = template.key;
    error = null;
    try {
      const suffix = Math.random().toString(36).slice(2, 7);
      const project = await api.post<Project>("/api/projects", {
        name: copy.label,
        slug: `${slugify(copy.label)}-${suffix}`,
        description: "",
      });
      const res = await api.post<TemplateInstallResult>(`/api/projects/${project.id}/templates/${template.key}/install`, {
        slug: slugify(copy.label),
      });
      result = { templateKey: template.key, res, projectId: project.id };
    } catch (e) {
      error = errMsg(e);
    } finally {
      installing = null;
    }
  }
</script>

{#if result}
  {@const r = result}
  {@const primary = r.res.deployments[0]}
  {@const credentials = Object.entries(r.res.credentials ?? {})}
  <Modal title="Your app is ready" {onClose}>
    {#snippet children()}
      <p>{SIMPLE_TEMPLATES[r.templateKey].label} is set up and starting.</p>
      {#if primary?.deploy_error}
        <div class="error-banner">Something went wrong getting it running: {primary.deploy_error}</div>
      {/if}
      {#if credentials.length > 0}
        <p class="text-dim" style="font-size: 13px">Save this now -- you'll need it to log in, and it won't be shown again.</p>
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
        <button class="btn btn-primary" onclick={() => primary && onAdded(r.projectId, primary.deployment_id)}>Done</button>
      </div>
    {/snippet}
  </Modal>
{:else}
  <Modal title="Add an app" {onClose} size="lg">
    {#snippet children(close)}
      {#if error}<div class="error-banner">{error}</div>{/if}
      {#if templates === null}
        <CenterLoading />
      {:else}
        <div class="template-grid">
          {#each templates as t (t.key)}
            {@const copy = SIMPLE_TEMPLATES[t.key]}
            <button
              type="button"
              class="card card-clickable template-card"
              style="opacity: {installing && installing !== t.key ? 0.5 : 1}; text-align: left; font: inherit; color: inherit"
              disabled={!!installing}
              onclick={() => install(t)}
            >
              <div class="template-card-top">
                <TemplateIcon templateKey={t.key} category={t.category} class="template-card-icon" />
                <div class="card-title" style="margin: 0">{copy.label}</div>
              </div>
              <p class="text-dim template-card-description">{installing === t.key ? "Setting it up..." : copy.blurb}</p>
            </button>
          {/each}
        </div>
      {/if}
      <div class="modal-actions">
        <button type="button" class="btn" onclick={close} disabled={!!installing}>Cancel</button>
      </div>
    {/snippet}
  </Modal>
{/if}
