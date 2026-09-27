import { LOADING } from "../lib/copy";

// The one full-page loading state -- a spinner paired with the swamp-voice
// caption instead of a bare spinner or a generic "Loading...", used
// everywhere a page waits on its first fetch.
export function CenterLoading({ label = LOADING }: { label?: string }) {
  return (
    <div className="center-loading center-loading-labeled">
      <div className="spinner" />
      <span className="text-faint">{label}</span>
    </div>
  );
}
