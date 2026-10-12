<script lang="ts">
  import { api, type Drive } from "../../api";
  import { errMsg } from "../../errors";
  import { slugify } from "../../slug";
  import Modal from "../Modal.svelte";

  let { drive, onClose, onShared }: { drive: Drive; onClose: () => void; onShared: () => void } = $props();

  function randomPassword(): string {
    const bytes = new Uint8Array(12);
    crypto.getRandomValues(bytes);
    return Array.from(bytes, (b) => b.toString(36).padStart(2, "0"))
      .join("")
      .slice(0, 16);
  }

  // svelte-ignore state_referenced_locally
  const defaultName = drive.label || drive.device.replace("/dev/", "");
  let shareName = $state(defaultName);
  let slug = $state(slugify(defaultName));
  let username = $state("share");
  let password = $state(randomPassword());
  let error = $state<string | null>(null);
  let busy = $state(false);

  async function submit(e: SubmitEvent) {
    e.preventDefault();
    busy = true;
    error = null;
    try {
      await api.post("/api/storage/shares", { drive_uuid: drive.uuid, slug, share_name: shareName, username, password });
      onShared();
    } catch (err) {
      error = errMsg(err);
    } finally {
      busy = false;
    }
  }
</script>

<Modal title="Share {drive.label || drive.device} as NAS" {onClose}>
  {#snippet children(close)}
    {#if error}<div class="error-banner">{error}</div>{/if}
    <form onsubmit={submit}>
      <div class="field">
        <label for="nas-share-name">Share name</label>
        <input
          id="nas-share-name"
          class="input"
          required
          value={shareName}
          oninput={(e) => {
            shareName = e.currentTarget.value;
            slug = slugify(shareName);
          }}
        />
        <div class="field-hint">Shown as smb://{window.location.hostname}/{shareName || "..."}</div>
      </div>
      <div class="field">
        <label for="nas-slug">Slug</label>
        <input id="nas-slug" class="input mono" required value={slug} oninput={(e) => (slug = slugify(e.currentTarget.value))} />
      </div>
      <div class="field">
        <label for="nas-username">SMB username</label>
        <input id="nas-username" class="input" required bind:value={username} />
      </div>
      <div class="field">
        <label for="nas-password">SMB password</label>
        <input id="nas-password" class="input mono" required minlength="8" bind:value={password} />
        <div class="field-hint">Shown once here -- write it down, Mangrove doesn't display it again.</div>
      </div>
      <div class="field-hint">
        The whole drive becomes writable over the network to anyone with these credentials. Creates a new deployment in the
        "Storage" project.
      </div>
      <div class="modal-actions">
        <button type="button" class="btn" onclick={close} disabled={busy}>Cancel</button>
        <button type="submit" class="btn btn-primary" disabled={busy}>{busy ? "Sharing..." : "Share"}</button>
      </div>
    </form>
  {/snippet}
</Modal>
