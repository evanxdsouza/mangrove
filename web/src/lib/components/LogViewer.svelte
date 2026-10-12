<script lang="ts">
  import { tick } from "svelte";

  // Live container logs over server-sent events, with a strip-chart trace
  // above: one tick per received line (amplitude from its length), a taller
  // danger tick for anything that reads as an error -- a real reading of
  // the stream, not a decorative waveform.
  let { serviceId }: { serviceId: number } = $props();

  const MAX_TICKS = 160;
  let lines = $state<string[]>([]);
  let connected = $state(false);
  let containerEl = $state<HTMLDivElement | null>(null);
  let autoScroll = true;

  $effect(() => {
    lines = [];
    const es = new EventSource(`/api/services/${serviceId}/logs/stream`);
    es.onopen = () => (connected = true);
    es.onerror = () => (connected = false);
    es.onmessage = async (evt) => {
      const next = lines.length > 2000 ? lines.slice(lines.length - 2000) : lines;
      lines = [...next, evt.data];
      await tick();
      if (autoScroll && containerEl) containerEl.scrollTop = containerEl.scrollHeight;
    };
    return () => es.close();
  });

  function onScroll() {
    if (!containerEl) return;
    autoScroll = containerEl.scrollHeight - containerEl.scrollTop - containerEl.clientHeight < 40;
  }

  const ticks = $derived(
    lines.slice(-MAX_TICKS).map((l) => ({
      h: Math.min(100, Math.max(14, (l.length % 46) + 14)),
      danger: /\b(error|fail(ed)?|fatal|panic)\b/i.test(l),
    })),
  );
</script>

<div class="log-viewer-frame">
  <div class="recorder-status {connected ? 'live' : ''}">
    <span>Stdout</span>
    <span class="flex" style="align-items: center; gap: 6px">
      <span class="rec-dot"></span>
      {connected ? "Recording" : "Connecting..."}
    </span>
  </div>
  <div>
    <div class="log-strip" aria-hidden="true">
      {#each ticks as t, i (i)}
        <span class="log-strip-tick {t.danger ? 'danger' : ''}" style:height="{t.h}%"></span>
      {/each}
    </div>
    <div class="log-viewer" bind:this={containerEl} onscroll={onScroll}>
      {#if lines.length === 0}
        <span style="color: var(--recorder-faint)">Waiting for log output...</span>
      {:else}
        {#each lines as l, i (i)}<div class="line">{l}</div>{/each}
      {/if}
    </div>
  </div>
</div>
