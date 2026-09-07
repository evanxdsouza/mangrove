import { useEffect, useMemo, useRef, useState } from "react";

const MAX_TICKS = 160;

export function LogViewer({ serviceId }: { serviceId: number }) {
  const [lines, setLines] = useState<string[]>([]);
  const [connected, setConnected] = useState(false);
  const containerRef = useRef<HTMLDivElement>(null);
  const autoScroll = useRef(true);

  useEffect(() => {
    setLines([]);
    const es = new EventSource(`/api/services/${serviceId}/logs/stream`);
    es.onopen = () => setConnected(true);
    es.onerror = () => setConnected(false);
    es.onmessage = (evt) => {
      setLines((prev) => {
        const next = prev.length > 2000 ? prev.slice(prev.length - 2000) : prev;
        return [...next, evt.data];
      });
    };
    return () => es.close();
  }, [serviceId]);

  useEffect(() => {
    if (autoScroll.current && containerRef.current) {
      containerRef.current.scrollTop = containerRef.current.scrollHeight;
    }
  }, [lines]);

  const onScroll = () => {
    const el = containerRef.current;
    if (!el) return;
    autoScroll.current = el.scrollHeight - el.scrollTop - el.clientHeight < 40;
  };

  // The strip-chart trace: one tick per received line (amplitude from its
  // length, a real if crude proxy for how much happened on that line), a
  // taller danger-colored tick for anything that reads as an error -- a
  // real reading of the stream, not a decorative waveform.
  const ticks = useMemo(
    () =>
      lines.slice(-MAX_TICKS).map((l) => ({
        h: Math.min(100, Math.max(14, (l.length % 46) + 14)),
        danger: /\b(error|fail(ed)?|fatal|panic)\b/i.test(l),
      })),
    [lines],
  );

  return (
    <div>
      <div className={`recorder-status ${connected ? "live" : ""}`}>
        <span className="rec-dot" />
        {connected ? "Recording" : "Connecting..."}
      </div>
      <div className="log-viewer-frame">
        <div className="log-strip" aria-hidden="true">
          {ticks.map((t, i) => (
            <span key={i} className={`log-strip-tick ${t.danger ? "danger" : ""}`} style={{ height: `${t.h}%` }} />
          ))}
        </div>
        <div className="log-viewer" ref={containerRef} onScroll={onScroll}>
          {lines.length === 0 ? (
            <span className="text-faint">Waiting for log output...</span>
          ) : (
            lines.map((l, i) => (
              <div className="line" key={i}>
                {l}
              </div>
            ))
          )}
        </div>
      </div>
    </div>
  );
}
