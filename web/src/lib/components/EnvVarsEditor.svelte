<script lang="ts">
  import { api, ApiError, type EnvVarEntry } from "../api";
  import Icon from "./Icon.svelte";

  // isAdmin gates the "Secret" checkbox -- setting a secret env var is
  // admin+ in the service's workspace server-side (internal/api/env.go).
  let { serviceId, isAdmin }: { serviceId: number; isAdmin: boolean } = $props();

  let vars = $state<EnvVarEntry[] | null>(null);
  let error = $state<string | null>(null);
  let key = $state("");
  let value = $state("");
  let isSecret = $state(false);
  let busy = $state(false);

  function load() {
    api
      .get<EnvVarEntry[]>(`/api/services/${serviceId}/env`)
      .then((v) => (vars = v ?? []))
      .catch((e) => (error = e instanceof ApiError ? e.message : "Failed to load env vars"));
  }

  $effect(() => {
    void serviceId;
    load();
  });

  async function add(e: SubmitEvent) {
    e.preventDefault();
    if (!key) return;
    busy = true;
    error = null;
    try {
      await api.put(`/api/services/${serviceId}/env/${encodeURIComponent(key)}`, { value, is_secret: isSecret });
      key = "";
      value = "";
      isSecret = false;
      load();
    } catch (err) {
      error = err instanceof ApiError ? err.message : "Failed to save";
    } finally {
      busy = false;
    }
  }

  async function remove(k: string) {
    await api.del(`/api/services/${serviceId}/env/${encodeURIComponent(k)}`);
    load();
  }
</script>

<div>
  {#if error}<div class="error-banner">{error}</div>{/if}
  {#if vars && vars.length > 0}
    <div class="kv-list" style="margin-bottom: 16px">
      {#each vars as v (v.key)}
        <div class="kv-row">
          <span class="kv-key">{v.key}</span>
          <span class="kv-value text-dim">{v.is_secret ? "•••••••• (encrypted)" : v.value}</span>
          <button class="btn btn-sm btn-danger" onclick={() => remove(v.key)}><Icon name="trash" /> Remove</button>
        </div>
      {/each}
    </div>
  {/if}
  <form onsubmit={add} class="form-row" style="align-items: flex-end">
    <div class="field" style="margin-bottom: 0">
      <label for="env-key-{serviceId}">Key</label>
      <input
        id="env-key-{serviceId}"
        class="input mono"
        value={key}
        oninput={(e) => (key = e.currentTarget.value.toUpperCase())}
        placeholder="API_KEY"
      />
    </div>
    <div class="field" style="margin-bottom: 0">
      <label for="env-value-{serviceId}">Value</label>
      <input id="env-value-{serviceId}" class="input mono" bind:value type={isSecret ? "password" : "text"} />
    </div>
    {#if isAdmin}
      <div class="field" style="margin-bottom: 0; flex: 0 0 auto">
        <label style="display: flex; align-items: center; gap: 6px; white-space: nowrap">
          <input type="checkbox" bind:checked={isSecret} />
          Secret
        </label>
      </div>
    {/if}
    <button class="btn" type="submit" disabled={busy || !key}><Icon name="plus" /> Add</button>
  </form>
</div>
