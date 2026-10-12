<script lang="ts">
  import { api, ApiError } from "../../api";
  import Icon from "../Icon.svelte";

  let { onAdded }: { onAdded: () => void } = $props();

  let email = $state("");
  let password = $state("");
  let role = $state<"owner" | "member">("member");
  let busy = $state(false);
  let error = $state<string | null>(null);

  async function submit(e: SubmitEvent) {
    e.preventDefault();
    if (!email || password.length < 8) return;
    busy = true;
    error = null;
    try {
      await api.post("/api/admin/users", { email, password, role });
      email = "";
      password = "";
      role = "member";
      onAdded();
    } catch (err) {
      error = err instanceof ApiError ? err.message : "Failed to add user";
    } finally {
      busy = false;
    }
  }
</script>

<form onsubmit={submit} class="form-row" style="margin-top: 14px; align-items: flex-end">
  {#if error}<div class="error-banner">{error}</div>{/if}
  <div class="field" style="margin-bottom: 0">
    <label for="team-email">Email</label>
    <input id="team-email" class="input" bind:value={email} placeholder="teammate@example.com" />
  </div>
  <div class="field" style="margin-bottom: 0">
    <label for="team-password">Initial password</label>
    <input id="team-password" class="input mono" type="password" bind:value={password} placeholder="min. 8 characters" />
  </div>
  <div class="field" style="margin-bottom: 0">
    <label for="team-role">Role</label>
    <select id="team-role" class="input" bind:value={role}>
      <option value="member">Member</option>
      <option value="owner">Owner</option>
    </select>
  </div>
  <button class="btn btn-sm" type="submit" disabled={busy || !email || password.length < 8}><Icon name="plus" /> Add</button>
</form>
