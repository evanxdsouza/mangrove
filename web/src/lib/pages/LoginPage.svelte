<script lang="ts">
  import { api, ApiError } from "../api";
  import Icon from "../components/Icon.svelte";
  import TideBanner from "../components/TideBanner.svelte";

  let { mode, onSuccess }: { mode: "setup" | "login"; onSuccess: () => void } = $props();

  let email = $state("");
  let password = $state("");
  let error = $state<string | null>(null);
  let busy = $state(false);

  async function submit(e: SubmitEvent) {
    e.preventDefault();
    error = null;
    busy = true;
    try {
      await api.post(mode === "setup" ? "/api/auth/setup" : "/api/auth/login", { email, password });
      onSuccess();
    } catch (err) {
      error = err instanceof ApiError ? err.message : "Something went wrong";
    } finally {
      busy = false;
    }
  }
</script>

<div class="login-shell">
  <div class="card login-card">
    <div class="login-card-banner"><TideBanner level="high" /></div>
    <div class="login-card-body">
      <Icon name="mangrove" class="login-mark" />
      <h1>{mode === "setup" ? "Welcome to Mangrove" : "Sign in"}</h1>
      <p class="subtitle">
        {mode === "setup" ? "Create the admin account to finish setting up this instance." : "Sign in to continue to your dashboard."}
      </p>

      {#if error}<div class="error-banner">{error}</div>{/if}

      <form onsubmit={submit}>
        <div class="field">
          <label for="email">Email</label>
          <input id="email" class="input" type="email" autocomplete="username" required bind:value={email} />
        </div>
        <div class="field">
          <label for="password">Password</label>
          <input
            id="password"
            class="input"
            type="password"
            autocomplete={mode === "setup" ? "new-password" : "current-password"}
            required
            minlength={mode === "setup" ? 8 : undefined}
            bind:value={password}
          />
          {#if mode === "setup"}<div class="field-hint">At least 8 characters.</div>{/if}
        </div>
        <button class="btn btn-primary" type="submit" disabled={busy} style="width: 100%; justify-content: center">
          {busy ? "..." : mode === "setup" ? "Create account" : "Sign in"}
        </button>
      </form>
    </div>
  </div>
</div>
