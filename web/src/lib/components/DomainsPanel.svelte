<script lang="ts">
  import { api, type CustomDomain } from "../api";
  import { errMsg } from "../errors";
  import Icon from "./Icon.svelte";

  // Lets an owner point a real domain at this deployment. In the default
  // mode, Mangrove programs a host-matched Caddy route once the domain's DNS
  // TXT record proves ownership, and Caddy provisions/renews TLS itself. In
  // "port" mode (config.Config.CustomDomainMode) there's no DNS step -- each
  // domain gets its own dedicated port, live immediately, to register in an
  // external domain->port dashboard (e.g. Hack Club Nest). See
  // internal/orchestrator/domains.go. canEdit (editor+) gates verifying;
  // isAdmin (admin+) gates adding/removing, matching the server-side split.
  let { deploymentId, canEdit, isAdmin }: { deploymentId: number; canEdit: boolean; isAdmin: boolean } = $props();

  let domains = $state<CustomDomain[] | null>(null);
  let hostname = $state("");
  let adding = $state(false);
  let verifyingId = $state<number | null>(null);
  let removingId = $state<number | null>(null);
  let error = $state<string | null>(null);

  function load() {
    api
      .get<CustomDomain[]>(`/api/deployments/${deploymentId}/domains`)
      .then((d) => (domains = d ?? []))
      .catch((e) => (error = errMsg(e)));
  }
  $effect(() => {
    void deploymentId;
    load();
  });

  async function add() {
    if (!hostname.trim()) return;
    adding = true;
    error = null;
    try {
      await api.post(`/api/deployments/${deploymentId}/domains`, { hostname: hostname.trim() });
      hostname = "";
      load();
    } catch (e) {
      error = errMsg(e);
    } finally {
      adding = false;
    }
  }

  async function verify(id: number) {
    verifyingId = id;
    error = null;
    try {
      await api.post(`/api/domains/${id}/verify`);
      load();
    } catch (e) {
      error = errMsg(e);
    } finally {
      verifyingId = null;
    }
  }

  async function remove(id: number) {
    removingId = id;
    error = null;
    try {
      await api.del(`/api/domains/${id}`);
      load();
    } catch (e) {
      error = errMsg(e);
    } finally {
      removingId = null;
    }
  }
</script>

<div class="card">
  <div class="card-title"><Icon name="globe" /> Domains</div>
  <p class="text-dim" style="margin-top: 0">
    Point a custom domain at this deployment. Mangrove terminates HTTPS for it automatically once verified, or (if this
    box is only reachable via a domain→port mapping, e.g. Hack Club Nest) gives the domain its own dedicated port to
    register there instead.
  </p>
  {#if error}<div class="error-banner">{error}</div>{/if}

  {#if domains && domains.length > 0}
    <table style="margin-bottom: 16px">
      <thead>
        <tr>
          <th>Hostname</th>
          <th>Status</th>
          {#if isAdmin}<th></th>{/if}
        </tr>
      </thead>
      <tbody>
        {#each domains as d (d.id)}
          <tr>
            <td>{d.hostname}</td>
            <td>
              {#if d.routing_mode === "port"}
                <div>
                  <span class="pill pill-green"><span class="pill-dot"></span>Live on port {d.port}</span>
                  <div class="field-hint" style="margin-top: 4px">
                    Register <code>{d.hostname}</code> → port <code>{d.port}</code> in your host's dashboard (e.g. Nest) to
                    point it here. No DNS verification needed.
                  </div>
                </div>
              {:else if d.verified}
                <span class="pill pill-green"><span class="pill-dot"></span>Verified</span>
              {:else}
                <div>
                  <span class="pill pill-yellow"><span class="pill-dot"></span>Pending verification</span>
                  <div class="field-hint" style="margin-top: 4px">
                    Add this TXT record on <code>{d.hostname}</code>, then verify:<br />
                    <code>mangrove-domain-verification={d.verification_token}</code>
                  </div>
                  {#if canEdit}
                    <button class="btn btn-sm" style="margin-top: 6px" onclick={() => verify(d.id)} disabled={verifyingId === d.id}>
                      {verifyingId === d.id ? "Verifying..." : "Verify"}
                    </button>
                  {/if}
                </div>
              {/if}
            </td>
            {#if isAdmin}
              <td>
                <button class="btn btn-sm btn-danger" onclick={() => remove(d.id)} disabled={removingId === d.id}>
                  <Icon name="trash" />
                  {removingId === d.id ? "Removing..." : "Remove"}
                </button>
              </td>
            {/if}
          </tr>
        {/each}
      </tbody>
    </table>
  {/if}
  {#if domains && domains.length === 0}
    <div class="empty-state"><p>No custom domains yet.</p></div>
  {/if}

  {#if isAdmin}
    <div class="field" style="display: flex; gap: 8px; align-items: flex-end">
      <div style="flex: 1">
        <label for="add-domain-hostname">Add a domain</label>
        <input id="add-domain-hostname" class="input" placeholder="app.example.com" bind:value={hostname} />
      </div>
      <button class="btn btn-sm" onclick={add} disabled={adding || !hostname.trim()}>
        <Icon name="plus" />
        {adding ? "Adding..." : "Add"}
      </button>
    </div>
  {/if}
</div>
