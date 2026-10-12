<script lang="ts">
  import { api, type Deployment } from "../../api";
  import { errMsg } from "../../errors";

  // On a staging deployment's own page: deploys production from the exact
  // commit currently running in staging, not production's branch tip --
  // see promoteDeployment in internal/api.
  let { projectId, deploymentId, deployment }: { projectId: number; deploymentId: number; deployment: Deployment } = $props();

  let busy = $state(false);
  let error = $state<string | null>(null);
  let done = $state(false);

  async function promote() {
    busy = true;
    error = null;
    done = false;
    try {
      await api.post(`/api/deployments/${deploymentId}/promote`, {});
      done = true;
    } catch (e) {
      error = errMsg(e);
    } finally {
      busy = false;
    }
  }
</script>

<div class="card">
  <div class="card-title">Promote to production</div>
  <p class="text-dim" style="margin-top: 0">
    Deploys production with the exact commit currently running here -- not just production's branch tip -- so what you've
    verified in staging is exactly what ships.
  </p>
  {#if error}<div class="error-banner">{error}</div>{/if}
  {#if done && !error}<div class="field-hint">Promotion started -- check the production deployment's history.</div>{/if}
  <div class="modal-actions" style="justify-content: flex-start">
    <a class="btn" href="/projects/{projectId}/deployments/{deployment.promotes_to_deployment_id}">View production deployment</a>
    <button type="button" class="btn btn-primary" onclick={promote} disabled={busy}>{busy ? "Promoting..." : "Promote to production"}</button>
  </div>
</div>
