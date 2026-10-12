<script lang="ts">
  import type { Snippet } from "svelte";
  import { page } from "$app/state";
  import { api } from "../api";
  import { currentUser, isOwner } from "../stores/user.svelte";
  import { setMode, uiMode } from "../stores/ui-mode.svelte";
  import Icon from "./Icon.svelte";
  import SidebarSearch from "./SidebarSearch.svelte";
  import StationSwitcher from "./StationSwitcher.svelte";

  let { onLogout, children }: { onLogout: () => void; children: Snippet } = $props();

  const SIDEBAR_COLLAPSE_KEY = "mangrove-sidebar-collapsed";
  function readStoredCollapsed(): boolean {
    try {
      return window.localStorage.getItem(SIDEBAR_COLLAPSE_KEY) === "1";
    } catch {
      return false;
    }
  }

  let mobileOpen = $state(false);
  let collapsed = $state(readStoredCollapsed());
  let critical = $state(false);

  const user = currentUser();
  const path = $derived(page.url.pathname);
  // Explicit membership, not "everything else" -- an unmatched path (a 404)
  // must not light up "Projects" as the active nav item.
  const onProjects = $derived(path === "/" || path.startsWith("/projects/"));
  const simple = $derived(uiMode.mode === "simple");

  function setCollapsed(next: boolean) {
    collapsed = next;
    try {
      window.localStorage.setItem(SIDEBAR_COLLAPSE_KEY, next ? "1" : "0");
    } catch {
      // localStorage unavailable -- still works for this session
    }
  }

  // A navigation closes the mobile drawer it was reached through.
  $effect(() => {
    void path;
    mobileOpen = false;
  });

  // The sidebar mark reacts to real host state: an owner-only poll of the
  // same resource budget Admin shows, tinting the mark when memory or disk
  // is above 90%. Every 5 minutes, not a tight poll -- the budget's
  // disk-usage scan is expensive (~7s observed), so this mirrors the
  // backend's own resource_sampler cadence. Members never poll (the
  // endpoint is owner-only server-side anyway).
  $effect(() => {
    if (!isOwner()) return;
    let cancelled = false;
    const check = () => {
      api
        .get<{ memory_used_mb: number; memory_ceiling_mb: number; disk_used_gb: number; disk_total_gb: number }>("/api/admin/resource-budget")
        .then((b) => {
          if (cancelled) return;
          const memFrac = b.memory_used_mb / Math.max(b.memory_ceiling_mb, 1);
          const diskFrac = b.disk_total_gb > 0 ? b.disk_used_gb / Math.max(b.disk_total_gb, 1) : 0;
          critical = memFrac > 0.9 || diskFrac > 0.9;
        })
        .catch(() => {});
    };
    check();
    const interval = setInterval(check, 300_000);
    return () => {
      cancelled = true;
      clearInterval(interval);
    };
  });
</script>

<div class="app-shell">
  <aside class="sidebar {mobileOpen ? 'mobile-open' : ''} {collapsed ? 'collapsed' : ''}">
    <div class="sidebar-topbar">
      <div class="sidebar-brand">
        <span style="display: inline-flex" title={critical ? "Host resources critical -- see Server health" : undefined}>
          <Icon name="mangrove" class="sidebar-brand-mark {critical ? 'sidebar-brand-mark-critical' : ''}" />
        </span>
        <span class="sidebar-brand-label">Mangrove</span>
        {#if critical}<span class="sidebar-critical-dot" aria-hidden="true"></span>{/if}
      </div>
      <button
        type="button"
        class="sidebar-menu-toggle"
        aria-label={mobileOpen ? "Close menu" : "Open menu"}
        aria-expanded={mobileOpen}
        onclick={() => (mobileOpen = !mobileOpen)}
      >
        <Icon name={mobileOpen ? "close" : "menu"} />
      </button>
    </div>

    <div class="sidebar-body">
      {#if critical}
        <a href="/server-health" class="sidebar-critical-note">Host memory or disk above 90%</a>
      {/if}
      {#if !simple}
        <StationSwitcher />
        <SidebarSearch {collapsed} onRequestExpand={() => setCollapsed(false)} />
        <div class="nav-section-label">Workspace</div>
      {/if}
      <a href="/" class="nav-link {onProjects ? 'active' : ''}" title={collapsed ? (simple ? "Your apps" : "Projects") : undefined}>
        <Icon name="ledger" />
        <span class="nav-link-label">{simple ? "Your apps" : "Projects"}</span>
      </a>

      <!-- Admin, server health, and storage stay technical-only: out of scope
           for a non-technical user by definition. Storage is also owner-only
           on the API itself (it mounts host drives), so its entry point is
           hidden for members too. -->
      {#if !simple}
        <div class="nav-section-label">Host</div>
        <div class="sidebar-rail-divider sidebar-collapsed-only" aria-hidden="true"></div>
        <a href="/admin" class="nav-link {path === '/admin' ? 'active' : ''}" title={collapsed ? "Admin" : undefined}>
          <Icon name="dials" />
          <span class="nav-link-label">Admin</span>
        </a>
        <a href="/server-health" class="nav-link {path === '/server-health' ? 'active' : ''}" title={collapsed ? "Server health" : undefined}>
          <Icon name="gauge" />
          <span class="nav-link-label">Server health</span>
        </a>
        {#if isOwner()}
          <a href="/storage" class="nav-link {path === '/storage' ? 'active' : ''}" title={collapsed ? "Storage" : undefined}>
            <Icon name="cabinet" />
            <span class="nav-link-label">Storage</span>
          </a>
        {/if}
      {/if}

      <div class="sidebar-footer">
        <div class="mode-toggle">
          <button
            type="button"
            class="mode-toggle-option {!simple ? 'active' : ''}"
            onclick={() => setMode("technical")}
            title="Full dashboard with technical detail"
          >
            Technical
          </button>
          <button
            type="button"
            class="mode-toggle-option {simple ? 'active' : ''}"
            onclick={() => setMode("simple")}
            title="Plain-language view for non-technical use"
          >
            Simple
          </button>
        </div>
        {#if simple}<div class="sidebar-mode-hint">Plain-language view for non-technical use</div>{/if}
        <a href="/settings" class="nav-link sidebar-collapsed-only {path === '/settings' ? 'active' : ''}" title="Settings" aria-label="Settings">
          <Icon name="gear" />
        </a>
        {#if !simple}
          <button
            type="button"
            class="sidebar-collapse-toggle"
            aria-label={collapsed ? "Expand sidebar" : "Collapse sidebar"}
            title={collapsed ? "Expand sidebar" : "Collapse sidebar"}
            onclick={() => setCollapsed(!collapsed)}
          >
            <Icon name="panel" />
            <span class="sidebar-collapse-toggle-label">Collapse</span>
          </button>
        {/if}
        <div class="sidebar-user">
          <span class="sidebar-user-email">{user.email ?? `user #${user.id}`}</span>
          <span class="sidebar-user-links">
            <a href="/settings" class={path === "/settings" ? "active" : undefined}>Settings</a>
            <button type="button" onclick={onLogout}>Log out</button>
          </span>
        </div>
      </div>
    </div>
  </aside>
  <main class="main">{@render children()}</main>
</div>
