<script lang="ts">
  import { onMount, setContext, type Snippet } from "svelte";
  import { createSpring, prefersReducedMotion, type SpringHandle } from "../spring";
  import Icon from "./Icon.svelte";

  let {
    title,
    onClose,
    size = "md",
    children,
  }: {
    title: string;
    onClose: () => void;
    /** "md" (460px) fits a form; "lg" (760px) is for anything that lays out
        a card grid inside itself (template gallery, GitHub repo picker). */
    size?: "md" | "lg";
    /** Receives `close`, which plays the same spring-out exit as the
        backdrop/Escape instead of unmounting abruptly. */
    children: Snippet<[() => void]>;
  } = $props();

  let progress = $state(0);
  let spring: SpringHandle | null = null;
  let closing = false;

  function requestClose() {
    if (closing) return;
    closing = true;
    if (!spring) {
      onClose();
      return;
    }
    spring.set(0);
  }

  // Nested components (e.g. a wizard step) can trigger the same exit.
  setContext("modal-close", requestClose);

  onMount(() => {
    if (prefersReducedMotion()) {
      progress = 1;
      return;
    }
    spring = createSpring(0, {
      damping: 1,
      response: 0.32,
      onUpdate: (v) => (progress = v),
      onComplete: () => {
        if (closing) onClose();
      },
    });
    spring.set(1);
    return () => spring?.stop();
  });
</script>

<svelte:window onkeydown={(e) => e.key === "Escape" && requestClose()} />

<!-- svelte-ignore a11y_no_static_element_interactions -->
<div
  class="modal-backdrop"
  style:opacity={progress}
  onmousedown={(e) => {
    if (e.target === e.currentTarget) requestClose();
  }}
>
  <div
    class="modal {size === 'lg' ? 'modal-lg' : ''}"
    role="dialog"
    aria-modal="true"
    aria-label={title}
    style:opacity={progress}
    style:transform="scale({0.96 + 0.04 * progress})"
  >
    <button type="button" class="modal-close" onclick={requestClose} aria-label="Close">
      <Icon name="close" />
    </button>
    <h2>{title}</h2>
    {@render children(requestClose)}
  </div>
</div>
