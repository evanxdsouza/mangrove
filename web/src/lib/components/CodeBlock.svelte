<script lang="ts">
  // The same terminal-box treatment as the log viewer for output that isn't
  // a live stream (a finished build failure, a run-command result).
  let { text, tone = "default" }: { text: string; tone?: "default" | "danger" } = $props();
</script>

<div class="log-viewer-frame">
  <pre class="code-block {tone === 'danger' ? 'danger' : ''}">{text}</pre>
</div>
