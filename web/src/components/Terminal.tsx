import { useEffect, useRef, useState } from "react";
import { Terminal as XTerm } from "@xterm/xterm";
import { FitAddon } from "@xterm/addon-fit";
import "@xterm/xterm/css/xterm.css";

type Status = "connecting" | "open" | "closed";

// ServiceTerminal opens an interactive shell inside a service's running
// container over a websocket (GET /api/services/{id}/terminal) and drives
// an xterm.js instance against it. Protocol: binary frames carry raw
// terminal bytes in both directions (keystrokes in, shell output back);
// the one text frame the client ever sends is a {"type":"resize"} control
// message, since raw shell output isn't guaranteed to be valid UTF-8 and
// so can't safely share a JSON-text channel.
export function ServiceTerminal({ serviceId }: { serviceId: number }) {
  const containerRef = useRef<HTMLDivElement>(null);
  const [status, setStatus] = useState<Status>("connecting");

  useEffect(() => {
    const el = containerRef.current;
    if (!el) return;

    const term = new XTerm({
      cursorBlink: true,
      fontFamily: "\"SF Mono\", \"Fira Code\", ui-monospace, Menlo, Consolas, monospace",
      fontSize: 13,
      theme: {
        background: "#05070c",
        foreground: "#c3d0e8",
      },
    });
    const fitAddon = new FitAddon();
    term.loadAddon(fitAddon);
    term.open(el);
    fitAddon.fit();

    setStatus("connecting");
    const proto = location.protocol === "https:" ? "wss:" : "ws:";
    const ws = new WebSocket(`${proto}//${location.host}/api/services/${serviceId}/terminal`);
    ws.binaryType = "arraybuffer";

    const sendResize = () => {
      if (ws.readyState !== WebSocket.OPEN) return;
      ws.send(JSON.stringify({ type: "resize", cols: term.cols, rows: term.rows }));
    };

    ws.onopen = () => {
      setStatus("open");
      sendResize();
      term.focus();
    };
    ws.onclose = () => setStatus("closed");
    ws.onerror = () => setStatus("closed");
    ws.onmessage = (evt) => {
      if (evt.data instanceof ArrayBuffer) {
        term.write(new Uint8Array(evt.data));
      }
    };

    const dataSub = term.onData((data) => {
      if (ws.readyState === WebSocket.OPEN) {
        ws.send(new TextEncoder().encode(data));
      }
    });

    const resizeObserver = new ResizeObserver(() => {
      fitAddon.fit();
      sendResize();
    });
    resizeObserver.observe(el);

    return () => {
      resizeObserver.disconnect();
      dataSub.dispose();
      ws.close();
      term.dispose();
    };
  }, [serviceId]);

  return (
    <div>
      <div className="flex-between" style={{ marginBottom: 8 }}>
        <span className="text-faint" style={{ fontSize: 12 }}>
          {status === "open" ? "Connected" : status === "connecting" ? "Connecting..." : "Disconnected"}
        </span>
      </div>
      <div className="terminal-panel" ref={containerRef} />
    </div>
  );
}
