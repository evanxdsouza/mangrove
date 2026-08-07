import { useEffect, useRef, useState } from "react";

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

  return (
    <div>
      <div className="flex-between" style={{ marginBottom: 8 }}>
        <span className="text-faint" style={{ fontSize: 12 }}>
          {connected ? "Live" : "Connecting..."}
        </span>
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
  );
}
