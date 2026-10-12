<script lang="ts">
  import { api, type Deployment } from "../../api";
  import { errMsg } from "../../errors";

  // The self-healing opt-in: a service whose health check fails repeatedly
  // gets restarted automatically, and a deploy that failed outright gets
  // retried (with backoff, up to 2 attempts) by the backend scheduler
  // (internal/scheduler/health.go, internal/scheduler/healer.go).
  let { deploymentId, deployment, onSaved, canEdit }: { deploymentId: number; deployment: Deployment | null; onSaved: () => void; canEdit: boolean } =
    $props();

  let enabled = $state(false);
  let busy = $state(false);
  let error = $state<string | null>(null);
  let saved = $state(false);

  let synced: boolean | null = null;
  $effect(() => {
    if (!deployment || deployment.self_heal_enabled === synced) return;
    synced = deployment.self_heal_enabled;
    enabled = deployment.self_heal_enabled;
  });

  async function save() {
    busy = true;
    error = null;
    saved = false;
    try {
      await api.post(`/api/deployments/${deploymentId}/self-heal`, { enabled });
      saved = true;
      onSaved();
    } catch (e) {
      error = errMsg(e);
    } finally {
      busy = false;
    }
  }
</script>

<div class="card">
  <div class="card-title">Self-healing</div>
  <p class="text-dim" style="margin-top: 0">
    Automatically restart this deployment's container if its health check fails repeatedly, and automatically retry a
    deploy that fails outright (up to 2 attempts, with backoff) instead of leaving it sitting failed until someone notices.
  </p>
  {#if error}<div class="error-banner">{error}</div>{/if}
  {#if !canEdit}<div class="field-hint">Only an editor or admin can change self-healing.</div>{/if}
  <div class="field">
    <label style="display: flex; align-items: center; gap: 8px">
      <input type="checkbox" bind:checked={enabled} disabled={!canEdit} />
      Self-heal automatically
    </label>
  </div>
  {#if deployment && deployment.auto_retry_count > 0 && deployment.status === "failed"}
    <div class="field-hint">Auto-retried {deployment.auto_retry_count} time{deployment.auto_retry_count === 1 ? "" : "s"} so far.</div>
  {/if}
  {#if canEdit}
    <button class="btn btn-sm" onclick={save} disabled={busy}>{busy ? "Saving..." : "Save"}</button>
  {/if}
  {#if saved}<div class="field-hint">Saved.</div>{/if}
</div>
