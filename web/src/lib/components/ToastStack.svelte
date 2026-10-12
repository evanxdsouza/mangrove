<script lang="ts">
  import { dismissToast, toasts } from "../stores/toast.svelte";
  import ToastItem from "./ToastItem.svelte";
</script>

<div class="toast-stack">
  {#each toasts as t (t.id)}
    <ToastItem message={t.message} onDone={() => dismissToast(t.id)} />
  {/each}
</div>
