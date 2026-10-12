<script lang="ts">
  // Self-hosted (no external font CDN -- this dashboard ships with the box
  // it manages) faces for the Intertidal voice: Figtree for UI and display,
  // JetBrains Mono for slugs, ports, env, and logs. Latin + Latin Extended
  // subsets only -- the full @fontsource bundles carry subsets the app never
  // uses, which only bloats the binary this dashboard is embedded into.
  import "@fontsource/figtree/latin-400.css";
  import "@fontsource/figtree/latin-ext-400.css";
  import "@fontsource/figtree/latin-500.css";
  import "@fontsource/figtree/latin-ext-500.css";
  import "@fontsource/figtree/latin-600.css";
  import "@fontsource/figtree/latin-ext-600.css";
  import "@fontsource/figtree/latin-700.css";
  import "@fontsource/figtree/latin-ext-700.css";
  import "@fontsource/jetbrains-mono/latin-400.css";
  import "@fontsource/jetbrains-mono/latin-ext-400.css";
  import "@fontsource/jetbrains-mono/latin-500.css";
  import "@fontsource/jetbrains-mono/latin-ext-500.css";
  import "$lib/styles.css";
  import "$lib/stores/theme.svelte";

  import type { Snippet } from "svelte";
  import { page } from "$app/state";
  import { api, type AuthStatus, type CurrentUser } from "$lib/api";
  import { session } from "$lib/stores/user.svelte";
  import { reloadWorkspaces } from "$lib/stores/workspaces.svelte";
  import Layout from "$lib/components/Layout.svelte";
  import ToastStack from "$lib/components/ToastStack.svelte";
  import ErrorFallback from "$lib/components/ErrorFallback.svelte";
  import LoginPage from "$lib/pages/LoginPage.svelte";

  let { children }: { children: Snippet } = $props();

  type AuthState = "loading" | "needs-setup" | "needs-login" | "authenticated";
  let auth = $state<AuthState>("loading");

  async function refresh() {
    try {
      const status = await api.get<AuthStatus>("/api/auth/status");
      if (status.setup_required) {
        session.user = null;
        auth = "needs-setup";
        return;
      }
      try {
        session.user = await api.get<CurrentUser>("/api/auth/me");
        auth = "authenticated";
        reloadWorkspaces();
      } catch {
        session.user = null;
        auth = "needs-login";
      }
    } catch {
      // API unreachable -- surface as needs-login so the user sees
      // something actionable rather than an infinite spinner.
      session.user = null;
      auth = "needs-login";
    }
  }

  async function logout() {
    try {
      await api.post("/api/auth/logout");
    } catch {
      // even if the request fails, drop the client-side session state
    }
    refresh();
  }

  refresh();
</script>

{#if auth === "loading"}
  <div class="center-loading"><div class="spinner"></div></div>
{:else if auth !== "authenticated"}
  <LoginPage mode={auth === "needs-setup" ? "setup" : "login"} onSuccess={refresh} />
{:else}
  <Layout onLogout={logout}>
    {#key page.url.pathname}
      <svelte:boundary onerror={(e) => console.error("Unhandled error rendering page:", e)}>
        {@render children()}
        {#snippet failed(error)}
          <ErrorFallback {error} />
        {/snippet}
      </svelte:boundary>
    {/key}
  </Layout>
{/if}

<ToastStack />
