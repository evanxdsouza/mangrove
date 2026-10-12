<script lang="ts">
  import { api, type WebhookEvent } from "../../api";
  import { errMsg } from "../../errors";
  import { fmtWhen } from "../../format";

  // Surfaces whether GitHub's webhook is actually registered and whether
  // recent deliveries got through -- otherwise "my push didn't deploy" has
  // no way to tell an unregistered webhook from a stale secret from a push
  // that simply matched no auto-deploying deployment.
  let { projectId, repo }: { projectId: number; repo: { repo_owner: string; repo_name: string; webhook_registered: boolean } } = $props();

  let events = $state<WebhookEvent[] | null>(null);
  let resyncing = $state(false);
  let resyncError = $state<string | null>(null);
  let instructions = $state<{ webhook_path: string; webhook_secret: string } | null>(null);

  $effect(() => {
    api
      .get<WebhookEvent[]>(`/api/projects/${projectId}/repo/webhook-events`)
      .then((e) => (events = e ?? []))
      .catch(() => (events = []));
  });

  async function resync() {
    resyncing = true;
    resyncError = null;
    try {
      const result = await api.post<{ webhook_registered: boolean; error?: string }>(`/api/projects/${projectId}/repo/resync-webhook`, {});
      if (!result.webhook_registered) {
        resyncError = result.error || "Couldn't register the webhook automatically -- use the manual setup instructions below.";
      }
    } catch (e) {
      resyncError = errMsg(e);
    } finally {
      resyncing = false;
    }
  }

  async function revealInstructions() {
    try {
      instructions = await api.get<{ webhook_path: string; webhook_secret: string }>(`/api/projects/${projectId}/repo/webhook-instructions`);
    } catch (e) {
      resyncError = errMsg(e);
    }
  }
</script>

<div style="margin-top: 16px; padding-top: 16px; border-top: 1px solid var(--border-soft)">
  <div class="flex-between" style="margin-bottom: 8px">
    <span class="text-dim" style="font-size: 13px">
      GitHub webhook:
      {#if repo.webhook_registered}
        <span class="pill pill-green" style="margin-left: 4px">registered</span>
      {:else}
        <span class="pill pill-yellow" style="margin-left: 4px">not confirmed</span>
      {/if}
    </span>
    <div class="flex gap-8">
      <button class="btn btn-sm" onclick={revealInstructions}>Setup instructions</button>
      <button class="btn btn-sm" onclick={resync} disabled={resyncing}>{resyncing ? "Resyncing..." : "Resync webhook"}</button>
    </div>
  </div>
  {#if resyncError}<div class="error-banner">{resyncError}</div>{/if}

  {#if instructions}
    <div class="kv-list" style="margin-bottom: 8px">
      <div class="kv-row"><span class="kv-key">Webhook URL</span><span class="kv-value mono">{window.location.origin + instructions.webhook_path}</span></div>
      <div class="kv-row"><span class="kv-key">Secret</span><span class="kv-value mono">{instructions.webhook_secret}</span></div>
      <div class="field-hint">
        Paste these into {repo.repo_owner}/{repo.repo_name} &rarr; Settings &rarr; Webhooks (content type: application/json,
        events: pushes, and pull requests if you want PR previews).
      </div>
    </div>
  {/if}

  {#if events && events.length > 0}
    <div class="kv-list">
      <div class="text-dim" style="font-size: 12px; margin-bottom: 4px">Recent deliveries</div>
      {#each events.slice(0, 8) as e (e.id)}
        <div class="kv-row">
          <span class="kv-key mono" style="font-size: 12px">{fmtWhen(e.received_at)}</span>
          <span class="kv-value text-dim" style="font-size: 12px">
            {e.event_type} &middot; {e.signature_valid ? "signed" : "bad signature"} &middot; {e.status.replace(/_/g, " ")}
          </span>
        </div>
      {/each}
    </div>
  {:else if events && events.length === 0}
    <div class="field-hint">No deliveries yet -- push to {repo.repo_owner}/{repo.repo_name} to see them here.</div>
  {/if}
</div>
