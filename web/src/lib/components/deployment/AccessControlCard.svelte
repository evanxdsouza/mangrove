<script lang="ts">
  import { api, type Deployment } from "../../api";
  import { errMsg } from "../../errors";

  let { deploymentId, deployment, onSaved, isAdmin }: { deploymentId: number; deployment: Deployment | null; onSaved: () => void; isAdmin: boolean } =
    $props();

  let isPublic = $state(false);
  let passwordProtected = $state(false);
  let password = $state("");
  let publicPaths = $state("");
  let busy = $state(false);
  let error = $state<string | null>(null);
  let saved = $state(false);

  // Re-sync from the server only when its values actually change, so the
  // page's 4-second poll never clobbers an edit in progress.
  let synced = "";
  $effect(() => {
    if (!deployment) return;
    const sig = JSON.stringify([deployment.is_public, deployment.password_protected, deployment.public_paths]);
    if (sig === synced) return;
    synced = sig;
    isPublic = deployment.is_public;
    passwordProtected = deployment.password_protected;
    publicPaths = (deployment.public_paths ?? []).join("\n");
  });

  async function save() {
    busy = true;
    error = null;
    saved = false;
    try {
      await api.post(`/api/deployments/${deploymentId}/access`, {
        is_public: isPublic,
        password_protected: passwordProtected,
        password,
        public_paths: publicPaths
          .split("\n")
          .map((p) => p.trim())
          .filter(Boolean),
      });
      password = "";
      saved = true;
      onSaved();
    } catch (e) {
      error = errMsg(e);
    } finally {
      busy = false;
    }
  }
</script>

<div class="card">
  <div class="card-title">Access control</div>
  <p class="text-dim" style="margin-top: 0">
    Enforced in front of the app itself, independent of any auth it has. A password-protected deployment shows visitors a
    gate page where they can sign in with a Mangrove account or enter the password below.
  </p>
  {#if error}<div class="error-banner">{error}</div>{/if}
  {#if !isAdmin}<div class="field-hint">Only an owner can change access control.</div>{/if}
  <div class="field">
    <label style="display: flex; align-items: center; gap: 8px">
      <input type="checkbox" bind:checked={isPublic} disabled={!isAdmin} />
      Public (expose on the assigned port)
    </label>
  </div>
  {#if isPublic}
    <div class="field">
      <label style="display: flex; align-items: center; gap: 8px">
        <input type="checkbox" bind:checked={passwordProtected} disabled={!isAdmin} />
        Password-protected
      </label>
    </div>
    {#if passwordProtected}
      <div class="field">
        <label for="access-password">Password</label>
        <input
          id="access-password"
          class="input"
          type="password"
          disabled={!isAdmin}
          bind:value={password}
          placeholder={deployment?.password_protected ? "Leave blank to keep the current password" : ""}
        />
      </div>
      <div class="field">
        <label for="access-public-paths">Unprotected pages</label>
        <textarea
          id="access-public-paths"
          class="input"
          rows="4"
          disabled={!isAdmin}
          bind:value={publicPaths}
          placeholder={"/pricing\n/docs/*"}
          spellcheck="false"
        ></textarea>
        <div class="field-hint">
          One path per line. These skip the password gate; everything else stays protected. A trailing * matches everything
          under that prefix (/docs/* ). A page's own CSS/JS/images need their paths listed too (e.g. /assets/*).
        </div>
      </div>
    {/if}
  {/if}
  {#if isAdmin}
    <button class="btn btn-sm" onclick={save} disabled={busy}>{busy ? "Saving..." : "Save"}</button>
  {/if}
  {#if saved}<div class="field-hint">Saved.</div>{/if}
</div>
