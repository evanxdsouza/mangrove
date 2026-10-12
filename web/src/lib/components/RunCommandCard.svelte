<script lang="ts">
  import { api, ApiError } from "../api";
  import Icon from "./Icon.svelte";

  // Runs a one-off command (e.g. a database migration) inside a service's
  // running container via POST /services/{id}/exec. Sent as exec-form argv
  // wrapped in `sh -c`, so the input stays one free-text field with no
  // host-side shell involved.
  let { serviceId, serviceName, showServiceName }: { serviceId: number; serviceName: string; showServiceName: boolean } = $props();

  interface ExecResponse {
    output: string;
    exit_code: number;
  }

  let command = $state("");
  let busy = $state(false);
  let error = $state<string | null>(null);
  let result = $state<ExecResponse | null>(null);

  async function run(e: SubmitEvent) {
    e.preventDefault();
    if (!command.trim()) return;
    busy = true;
    error = null;
    result = null;
    try {
      result = await api.post<ExecResponse>(`/api/services/${serviceId}/exec`, { command: ["sh", "-c", command] });
    } catch (err) {
      error = err instanceof ApiError ? err.message : "Failed to run command";
    } finally {
      busy = false;
    }
  }
</script>

<div class="card">
  <div class="card-title">Run command{showServiceName ? ` — ${serviceName}` : ""}</div>
  <p class="text-dim" style="margin-top: 0">
    Runs inside the service's currently running container -- e.g. a database migration
    (<code class="mono">npm run migrate</code>, <code class="mono">rails db:migrate</code>). The container must be running.
  </p>
  {#if error}<div class="error-banner">{error}</div>{/if}
  <form onsubmit={run} class="form-row" style="align-items: flex-end">
    <div class="field" style="margin-bottom: 0; flex: 1">
      <label for="run-command-{serviceId}">Command</label>
      <input id="run-command-{serviceId}" class="input mono" bind:value={command} placeholder="npm run migrate" />
    </div>
    <button class="btn" type="submit" disabled={busy || !command.trim()}>
      <Icon name="terminal" />
      {busy ? "Running..." : "Run"}
    </button>
  </form>
  {#if result}
    <div style="margin-top: 12px">
      <div class="field-hint">Exit code: {result.exit_code}</div>
      <pre class="mono" style="white-space: pre-wrap; overflow-x: auto">{result.output || "(no output)"}</pre>
    </div>
  {/if}
</div>
