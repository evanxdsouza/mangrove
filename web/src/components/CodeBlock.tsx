// The one component for "a block of raw machine output the operator might
// need to read closely" -- build failures, command output, error detail.
// LogViewer owns the *live streaming* case (its strip-chart + recorder
// status only make sense for an active connection); CodeBlock is the same
// terminal-box visual treatment for output that's already finished, so the
// two content types (a live tail vs. a finished error dump) never diverge
// into "one gets a real terminal box, the other gets bare paragraph text"
// the way History's build-failure text used to next to Logs' LogViewer.
export function CodeBlock({ children, tone = "default" }: { children: string; tone?: "default" | "danger" }) {
  return (
    <div className="log-viewer-frame">
      <pre className={`code-block ${tone === "danger" ? "danger" : ""}`}>{children}</pre>
    </div>
  );
}
