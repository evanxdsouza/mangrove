<script lang="ts">
  import { onMount } from "svelte";
  import { createSpring, prefersReducedMotion } from "../spring";
  import Icon from "./Icon.svelte";

  let { message, onDone }: { message: string; onDone: () => void } = $props();
  let progress = $state(0);

  onMount(() => {
    if (prefersReducedMotion()) {
      progress = 1;
      const timer = setTimeout(onDone, 5000);
      return () => clearTimeout(timer);
    }
    let closing = false;
    const spring = createSpring(0, {
      damping: 0.78,
      response: 0.4,
      onUpdate: (v) => (progress = v),
      onComplete: () => {
        if (closing) onDone();
      },
    });
    spring.set(1);
    const timer = setTimeout(() => {
      closing = true;
      spring.set(0, 0);
    }, 4200);
    return () => {
      clearTimeout(timer);
      spring.stop();
    };
  });
</script>

<div
  class="toast"
  role="status"
  style:opacity={progress}
  style:transform="translateY({(1 - progress) * 14}px) scale({0.94 + 0.06 * progress})"
>
  <Icon name="mangrove" class="toast-icon" />
  <span>{message}</span>
</div>
