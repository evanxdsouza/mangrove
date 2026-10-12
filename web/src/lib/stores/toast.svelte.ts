// Rare, on-brand milestone moments (first deploy, first rollback) surface
// as a small toast -- kept deliberately rare by callers (see
// lib/milestones.ts) so this stays a delight, not a notification feed.
// Rendered by components/ToastStack.svelte in the root layout.
export interface ToastEntry {
  id: number;
  message: string;
}

let nextId = 1;

export const toasts = $state<ToastEntry[]>([]);

export function showToast(message: string) {
  toasts.push({ id: nextId++, message });
}

export function dismissToast(id: number) {
  const i = toasts.findIndex((t) => t.id === id);
  if (i >= 0) toasts.splice(i, 1);
}
