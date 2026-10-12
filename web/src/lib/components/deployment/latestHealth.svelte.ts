import { api, type HealthCheckEntry } from "../../api";

// Shared by ServiceCard and ServiceStatRow so a service's latest health
// reading is polled the same way regardless of which layout shows it.
// Call during component init; the poll stops with the component.
export function latestHealth(getServiceId: () => number, getIsStatic: () => boolean): { current: HealthCheckEntry | null } {
  const state = $state<{ current: HealthCheckEntry | null }>({ current: null });
  $effect(() => {
    const serviceId = getServiceId();
    if (getIsStatic()) return; // no container, no health checks to fetch
    const load = () =>
      api
        .get<HealthCheckEntry[]>(`/api/services/${serviceId}/health?limit=1`)
        .then((h) => (state.current = h && h.length > 0 ? h[0] : null))
        .catch(() => (state.current = null));
    load();
    const interval = setInterval(load, 4000);
    return () => clearInterval(interval);
  });
  return state;
}
