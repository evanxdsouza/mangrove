<script lang="ts">
  import type { Deployment, Project } from "../../api";
  import { fmtWhen } from "../../format";
  import TideGauge from "../TideGauge.svelte";

  // The Projects list's persistent side rail: the most recently deployed
  // things across every loaded project -- built entirely from data the page
  // already fetched, never a separate endpoint.
  let { projects, deploymentsByProject }: { projects: Project[]; deploymentsByProject: Record<number, Deployment[]> } = $props();

  const projectNameById = $derived(new Map(projects.map((p) => [p.id, p.name])));
  const items = $derived(
    Object.entries(deploymentsByProject)
      .flatMap(([projectId, deps]) => deps.map((d) => ({ ...d, projectId: Number(projectId) })))
      .filter((d) => d.last_deployed_at)
      .sort((a, b) => new Date(b.last_deployed_at!).getTime() - new Date(a.last_deployed_at!).getTime())
      .slice(0, 8),
  );
</script>

<div class="card activity-panel">
  <div class="card-title">Recent activity</div>
  {#if items.length === 0}
    <p class="text-dim" style="margin: 0">Nothing deployed yet -- recent deployments across every project will show up here.</p>
  {:else}
    <div class="activity-list">
      {#each items as d (d.id)}
        <a href="/projects/{d.projectId}/deployments/{d.id}" class="activity-item">
          <span class="activity-item-rail"><TideGauge status={d.status} size="sm" /></span>
          <span class="activity-item-body">
            <span class="activity-item-name">{d.name}</span>
            <span class="text-dim activity-item-meta">{projectNameById.get(d.projectId) ?? "..."} &middot; {fmtWhen(d.last_deployed_at)}</span>
          </span>
        </a>
      {/each}
    </div>
  {/if}
</div>
