<script lang="ts">
  import { goto } from "$app/navigation";
  import { api, ApiError, type Deployment, type Project } from "../../api";
  import { EMPTY_APPS } from "../../copy";
  import Icon from "../../components/Icon.svelte";
  import TideGauge from "../../components/TideGauge.svelte";
  import CenterLoading from "../../components/CenterLoading.svelte";
  import GithubDeployWizard from "../../components/GithubDeployWizard.svelte";
  import AddAppModal from "../../components/simple/AddAppModal.svelte";
  import PlainStatusPill from "./PlainStatusPill.svelte";

  // Simple mode's home page: every deployment across every project,
  // flattened into one plain "your apps" list -- the project grouping
  // concept from technical mode never surfaces here.
  interface FlatApp {
    deployment: Deployment;
    projectId: number;
  }

  let apps = $state<FlatApp[] | null>(null);
  let error = $state<string | null>(null);
  let showAdd = $state(false);
  let showGithub = $state(false);

  function load() {
    api
      .get<Project[]>("/api/projects")
      .then(async (projects) => {
        const lists = await Promise.all(
          (projects ?? []).map((p) =>
            api
              .get<Deployment[]>(`/api/projects/${p.id}/deployments`)
              .then((deps) => (deps ?? []).map((d): FlatApp => ({ deployment: d, projectId: p.id })))
              .catch(() => [] as FlatApp[]),
          ),
        );
        apps = lists.flat();
        error = null;
      })
      .catch((e) => (error = e instanceof ApiError ? e.message : "Couldn't load your apps"));
  }

  $effect(() => {
    load();
    const interval = setInterval(load, 4000);
    return () => clearInterval(interval);
  });
</script>

<div class="page-header">
  <div>
    <h1>Your apps</h1>
    <p>Everything you've set up, in one place.</p>
  </div>
  <div class="flex gap-8">
    <button class="btn" onclick={() => (showGithub = true)}><Icon name="branch" /> From GitHub</button>
    <button class="btn btn-primary" onclick={() => (showAdd = true)}><Icon name="plus" /> Add an app</button>
  </div>
</div>

{#if error}<div class="error-banner">{error}</div>{/if}

{#if apps === null}
  <CenterLoading />
{:else if apps.length === 0}
  <div class="card empty-state">
    <p>{EMPTY_APPS}</p>
    <div class="field-hint">Add one to get started -- pick a template or connect a GitHub repo above.</div>
  </div>
{:else}
  <div class="grid grid-2">
    {#each apps as { deployment, projectId } (deployment.id)}
      <a class="card card-clickable" href="/projects/{projectId}/deployments/{deployment.id}">
        <div class="flex-between" style="margin-bottom: 0; gap: 10px">
          <span class="flex gap-8" style="align-items: center; min-width: 0">
            <TideGauge status={deployment.status} size="sm" />
            <span class="card-title" style="margin: 0">{deployment.name}</span>
          </span>
          <PlainStatusPill status={deployment.status} />
        </div>
      </a>
    {/each}
  </div>
{/if}

{#if showAdd}
  <AddAppModal
    onClose={() => (showAdd = false)}
    onAdded={(projectId, deploymentId) => {
      showAdd = false;
      goto(`/projects/${projectId}/deployments/${deploymentId}`);
    }}
  />
{/if}

{#if showGithub}
  <GithubDeployWizard
    onClose={() => (showGithub = false)}
    onDeployed={(projectId, deploymentId) => {
      showGithub = false;
      goto(`/projects/${projectId}/deployments/${deploymentId}`);
    }}
  />
{/if}
