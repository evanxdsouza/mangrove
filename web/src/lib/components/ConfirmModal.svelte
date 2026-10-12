<script lang="ts">
  import Modal from "./Modal.svelte";
  import Icon from "./Icon.svelte";
  import { ApiError } from "../api";

  // Confirmation for destructive actions (delete project/deployment): these
  // tear down live containers and Caddy routes, so they get an explicit
  // confirm step rather than firing on the first click.
  let {
    title,
    body,
    confirmLabel,
    onConfirm,
    onClose,
  }: { title: string; body: string; confirmLabel: string; onConfirm: () => Promise<void>; onClose: () => void } = $props();

  let busy = $state(false);
  let error = $state<string | null>(null);

  async function handleConfirm() {
    busy = true;
    error = null;
    try {
      await onConfirm();
    } catch (e) {
      error = e instanceof ApiError ? e.message : "Something went wrong";
      busy = false;
    }
  }
</script>

<Modal {title} {onClose}>
  {#snippet children(close)}
    {#if error}<div class="error-banner">{error}</div>{/if}
    <p style="display: flex; gap: 10px; align-items: flex-start">
      <Icon name="alert" style="width: 17px; height: 17px; color: var(--red-bright); flex-shrink: 0; margin-top: 1px" />
      <span>{body}</span>
    </p>
    <div class="modal-actions">
      <button type="button" class="btn" onclick={close} disabled={busy}>Cancel</button>
      <button type="button" class="btn btn-danger" onclick={handleConfirm} disabled={busy}>
        <Icon name="trash" />
        {busy ? "Deleting..." : confirmLabel}
      </button>
    </div>
  {/snippet}
</Modal>
