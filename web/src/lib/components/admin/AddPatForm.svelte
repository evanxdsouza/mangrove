<script lang="ts">
  import { api, ApiError } from "../../api";
  import Icon from "../Icon.svelte";

  let { onAdded }: { onAdded: () => void } = $props();

  let label = $state("");
  let token = $state("");
  let busy = $state(false);
  let error = $state<string | null>(null);

  async function submit(e: SubmitEvent) {
    e.preventDefault();
    if (!label || !token) return;
    busy = true;
    error = null;
    try {
      await api.post("/api/github/pats", { label, token });
      label = "";
      token = "";
      onAdded();
    } catch (err) {
      error = err instanceof ApiError ? err.message : "Failed to add token";
    } finally {
      busy = false;
    }
  }
</script>

<form onsubmit={submit} class="form-row" style="margin-top: 14px; align-items: flex-end">
  {#if error}<div class="error-banner">{error}</div>{/if}
  <div class="field" style="margin-bottom: 0">
    <label for="pat-label">Label</label>
    <input id="pat-label" class="input" bind:value={label} placeholder="personal" />
  </div>
  <div class="field" style="margin-bottom: 0">
    <label for="pat-token">Token</label>
    <input id="pat-token" class="input mono" type="password" bind:value={token} placeholder="ghp_..." />
  </div>
  <button class="btn btn-sm" type="submit" disabled={busy || !label || !token}><Icon name="plus" /> Add</button>
</form>
