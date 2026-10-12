<script lang="ts">
  import { api } from "../../api";

  let { onReserved }: { onReserved: () => void } = $props();

  let port = $state<number | null>(null);
  let note = $state("");
  let busy = $state(false);

  async function submit(e: SubmitEvent) {
    e.preventDefault();
    if (!port) return;
    busy = true;
    try {
      await api.post("/api/admin/ports", { port: Number(port), note });
      port = null;
      note = "";
      onReserved();
    } finally {
      busy = false;
    }
  }
</script>

<form onsubmit={submit} class="form-row" style="margin-top: 14px; align-items: flex-end">
  <div class="field" style="margin-bottom: 0">
    <label for="reserve-port">Reserve port</label>
    <input id="reserve-port" class="input mono" type="number" bind:value={port} placeholder="8080" />
  </div>
  <div class="field" style="margin-bottom: 0">
    <label for="reserve-port-note">Note</label>
    <input id="reserve-port-note" class="input" bind:value={note} placeholder="used by X outside Mangrove" />
  </div>
  <button class="btn btn-sm" type="submit" disabled={busy || !port}>Reserve</button>
</form>
