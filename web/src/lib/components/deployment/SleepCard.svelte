<script lang="ts">
  import { api, type Deployment } from "../../api";
  import { errMsg } from "../../errors";

  // The idle-sleep opt-in: a deployment with no traffic for
  // sleep_idle_minutes is stopped by the backend scheduler
  // (internal/scheduler/sleeper.go) and auto-wakes on its next visit,
  // showing a "waking up" page meanwhile (internal/api/gate.go).
  let { deploymentId, deployment, onSaved, canEdit }: { deploymentId: number; deployment: Deployment | null; onSaved: () => void; canEdit: boolean } =
    $props();

  let enabled = $state(false);
  let idleMinutes = $state(30);
  let busy = $state(false);
  let error = $state<string | null>(null);
  let saved = $state(false);

  let synced = "";
  $effect(() => {
    if (!deployment) return;
    const sig = JSON.stringify([deployment.sleep_enabled, deployment.sleep_idle_minutes]);
    if (sig === synced) return;
    synced = sig;
    enabled = deployment.sleep_enabled;
    idleMinutes = deployment.sleep_idle_minutes;
  });

  async function save() {
    busy = true;
    error = null;
    saved = false;
    try {
      await api.post(`/api/deployments/${deploymentId}/sleep`, { enabled, idle_minutes: idleMinutes });
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
  <div class="card-title">Idle sleep</div>
  <p class="text-dim" style="margin-top: 0">
    Stop this deployment's container automatically after a period with no traffic, and wake it back up the moment someone
    visits -- they'll see a brief "waking up" page while it starts.
  </p>
  {#if error}<div class="error-banner">{error}</div>{/if}
  {#if !canEdit}<div class="field-hint">Only an editor or admin can change idle sleep.</div>{/if}
  <div class="field">
    <label style="display: flex; align-items: center; gap: 8px">
      <input type="checkbox" bind:checked={enabled} disabled={!canEdit} />
      Sleep when idle
    </label>
  </div>
  {#if enabled}
    <div class="field">
      <label for="sleep-idle-minutes">Idle minutes before sleeping</label>
      <input
        id="sleep-idle-minutes"
        class="input"
        type="number"
        min="1"
        max="1440"
        style="max-width: 140px"
        disabled={!canEdit}
        value={idleMinutes}
        oninput={(e) => (idleMinutes = Math.max(1, Number(e.currentTarget.value) || 1))}
      />
      {#if deployment?.status === "sleeping"}<div class="field-hint">Currently asleep -- the next visit will wake it.</div>{/if}
    </div>
  {/if}
  {#if canEdit}
    <button class="btn btn-sm" onclick={save} disabled={busy}>{busy ? "Saving..." : "Save"}</button>
  {/if}
  {#if saved}<div class="field-hint">Saved.</div>{/if}
</div>
