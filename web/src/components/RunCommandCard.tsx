import { useState, type FormEvent } from "react";
import { api, ApiError } from "../api";

interface ExecResponse {
  output: string;
  exit_code: number;
}

// RunCommandCard runs a one-off command (e.g. a database migration) inside
// a service's currently-running container via POST /services/{id}/exec.
// The command is sent as exec-form argv (no shell involved on the host
// side), matching Service.Command/RunSpec.Command elsewhere -- wrapping in
// `sh -c` ourselves is what lets the input stay a single free-text field
// without losing that property.
export function RunCommandCard({
  serviceId,
  serviceName,
  showServiceName,
}: {
  serviceId: number;
  serviceName: string;
  showServiceName: boolean;
}) {
  const [command, setCommand] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [result, setResult] = useState<ExecResponse | null>(null);

  const run = async (e: FormEvent) => {
    e.preventDefault();
    if (!command.trim()) return;
    setBusy(true);
    setError(null);
    setResult(null);
    try {
      const res = await api.post<ExecResponse>(`/api/services/${serviceId}/exec`, {
        command: ["sh", "-c", command],
      });
      setResult(res);
    } catch (e) {
      setError(e instanceof ApiError ? e.message : "Failed to run command");
    } finally {
      setBusy(false);
    }
  };

  return (
    <div className="card">
      <div className="card-title">Run command{showServiceName ? ` — ${serviceName}` : ""}</div>
      <p className="text-dim" style={{ marginTop: 0 }}>
        Runs inside the service's currently running container -- e.g. a database migration
        (<code className="mono">npm run migrate</code>, <code className="mono">rails db:migrate</code>). The
        container must be running.
      </p>
      {error && <div className="error-banner">{error}</div>}
      <form onSubmit={run} className="form-row" style={{ alignItems: "flex-end" }}>
        <div className="field" style={{ marginBottom: 0, flex: 1 }}>
          <label htmlFor={`run-command-${serviceId}`}>Command</label>
          <input
            id={`run-command-${serviceId}`}
            className="input mono"
            value={command}
            onChange={(e) => setCommand(e.target.value)}
            placeholder="npm run migrate"
          />
        </div>
        <button className="btn" type="submit" disabled={busy || !command.trim()}>
          {busy ? "Running..." : "Run"}
        </button>
      </form>
      {result && (
        <div style={{ marginTop: 12 }}>
          <div className="field-hint">Exit code: {result.exit_code}</div>
          <pre className="mono" style={{ whiteSpace: "pre-wrap", overflowX: "auto" }}>
            {result.output || "(no output)"}
          </pre>
        </div>
      )}
    </div>
  );
}
