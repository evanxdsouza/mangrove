<script lang="ts">
  import { api, ApiError } from "../api";
  import { setTheme, THEMES, themeState } from "../stores/theme.svelte";
  import Icon from "../components/Icon.svelte";

  let currentPassword = $state("");
  let newPassword = $state("");
  let confirmPassword = $state("");
  let busy = $state(false);
  let error = $state<string | null>(null);
  let success = $state(false);

  const mismatch = $derived(confirmPassword.length > 0 && newPassword !== confirmPassword);
  const canSubmit = $derived(currentPassword.length > 0 && newPassword.length >= 8 && newPassword === confirmPassword);

  async function submit(e: SubmitEvent) {
    e.preventDefault();
    if (!canSubmit) return;
    busy = true;
    error = null;
    success = false;
    try {
      await api.post("/api/auth/change-password", { current_password: currentPassword, new_password: newPassword });
      currentPassword = "";
      newPassword = "";
      confirmPassword = "";
      success = true;
    } catch (err) {
      error = err instanceof ApiError ? err.message : "Failed to change password";
    } finally {
      busy = false;
    }
  }
</script>

<div class="page-header">
  <div>
    <h1>Settings</h1>
    <p>Manage your account.</p>
  </div>
  <Icon name="gear" />
</div>

<div class="card">
  <div class="card-title">Instrument metal</div>
  <p class="text-dim" style="margin-top: 0">
    Pick which metal trims the panel -- the sidebar's small section labels re-color to match. The deep-green sidebar,
    leaf-green buttons, and tide status colors stay put regardless.
  </p>
  <div class="theme-grid">
    {#each THEMES as t (t.id)}
      <button type="button" class="theme-swatch {themeState.theme === t.id ? 'active' : ''}" onclick={() => setTheme(t.id)} style="--brass: {t.accent}">
        <Icon name="mangrove" />
        {t.label}
      </button>
    {/each}
  </div>
</div>

<div class="card">
  <div class="card-title">Change password</div>
  <form onsubmit={submit} style="max-width: 360px; margin-top: 14px">
    {#if error}<div class="error-banner">{error}</div>{/if}
    {#if success}<div class="field-hint">Password updated.</div>{/if}
    <div class="field">
      <label for="current-password">Current password</label>
      <input id="current-password" class="input mono" type="password" autocomplete="current-password" bind:value={currentPassword} />
    </div>
    <div class="field">
      <label for="new-password">New password</label>
      <input id="new-password" class="input mono" type="password" autocomplete="new-password" bind:value={newPassword} placeholder="min. 8 characters" />
    </div>
    <div class="field">
      <label for="confirm-password">Confirm new password</label>
      <input
        id="confirm-password"
        class="input mono"
        type="password"
        autocomplete="new-password"
        aria-invalid={mismatch}
        bind:value={confirmPassword}
      />
      {#if mismatch}<div class="field-hint">Passwords don't match.</div>{/if}
    </div>
    <button class="btn btn-primary btn-sm" type="submit" disabled={busy || !canSubmit}>Update password</button>
  </form>
</div>
