<script lang="ts">
  // A real 404: an unrecognized path says so instead of silently falling
  // through to the Projects list, which made a bad link look like it worked.
  let { path }: { path: string } = $props();
</script>

<div class="card empty-state" style="margin-top: 60px">
  <p>Nothing's rooted at this address.</p>
  <div class="field-hint">No page matches <code class="mono">{path}</code>.</div>
  <div style="margin-top: 16px"><a href="/" class="btn btn-primary">Back to solid ground</a></div>
</div>
