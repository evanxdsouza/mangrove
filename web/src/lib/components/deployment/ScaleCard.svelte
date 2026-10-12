<script lang="ts">
  import { api, type Deployment } from "../../api";
  import { errMsg } from "../../errors";

  let { deploymentId, deployment }: { deploymentId: number; deployment: Deployment } = $props();

  // svelte-ignore state_referenced_locally
  let value = $state(deployment.replicas);
  let busy = $state(false);
  let saved = $state(false);
  let error = $state<string | null>(null);

  async function apply() {
    busy = true;
    saved = false;
    error = null;
    try {
      await api.post(`/api/deployments/${deploymentId}/scale`, { replicas: value });
      saved = true;
    } catch (e) {
      error = errMsg(e);
    } finally {
      busy = false;
    }
  }
</script>

{#if deployment.build_strategy !== "compose" && deployment.build_strategy !== "static"}
  <div class="card">
    <div class="card-title">Replicas</div>
    <p class="text-dim" style="margin-top: 0">
      Number of containers of this image running behind one load-balanced route. Changing this triggers a redeploy.
    </p>
    {#if error}<div class="error-banner">{error}</div>{/if}
    <div class="form-row" style="align-items: flex-end">
      <div class="field" style="margin-bottom: 0">
        <label for="scale-replicas">Replicas</label>
        <input
          id="scale-replicas"
          class="input"
          type="number"
          min="1"
          max="32"
          {value}
          oninput={(e) => (value = Math.max(1, Number(e.currentTarget.value) || 1))}
        />
      </div>
      <button class="btn btn-sm" onclick={apply} disabled={busy || value === deployment.replicas}>{busy ? "Scaling..." : "Scale"}</button>
    </div>
    {#if saved}<div class="field-hint">Scaling. Watch the deployment status while it redeploys.</div>{/if}
  </div>
{/if}
