import { useEffect, useState } from "react";
import { api } from "../api";

interface SystemHealth {
  hostname: string;
  kernel: string;
  uptime_seconds: number;
  load_avg_1: number;
  load_avg_5: number;
  load_avg_15: number;
  cpu_count: number;
  cpu_percent: number;
  memory_total_gb: number;
  memory_used_gb: number;
  memory_used_pct: number;
  swap_total_gb: number;
  swap_used_gb: number;
  disk_total_gb: number;
  disk_used_gb: number;
  disk_used_pct: number;
  processes: number;
  running_process: number;
}

function fmtGB(gb: number): string {
  return `${gb.toFixed(1)} GB`;
}

function fmtDuration(seconds: number): string {
  const d = Math.floor(seconds / 86400);
  const h = Math.floor((seconds % 86400) / 3600);
  const m = Math.floor((seconds % 3600) / 60);
  const parts: string[] = [];
  if (d > 0) parts.push(`${d}d`);
  if (h > 0 || d > 0) parts.push(`${h}h`);
  parts.push(`${m}m`);
  return parts.join(" ");
}

export function ServerHealthPage() {
  const [health, setHealth] = useState<SystemHealth | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    const load = () => {
      api
        .get<SystemHealth>("/api/admin/system-health")
        .then((h) => {
          if (!cancelled) {
            setHealth(h);
            setError(null);
          }
        })
        .catch((e) => {
          if (!cancelled) setError(e instanceof Error ? e.message : "Failed to load server health");
        });
    };
    load();
    const timer = setInterval(load, 5000);
    return () => {
      cancelled = true;
      clearInterval(timer);
    };
  }, []);

  return (
    <>
      <div className="page-header">
        <div>
          <h1>Server health</h1>
          <p>Live status of the whole host, not just Mangrove's containers.</p>
        </div>
      </div>

      {error && <div className="error-banner">{error}</div>}

      {health ? (
        <>
          <div className="card">
            <div className="card-title">System</div>
            <table>
              <tbody>
                <tr>
                  <td className="text-dim">Host</td>
                  <td className="mono">{health.hostname}</td>
                </tr>
                <tr>
                  <td className="text-dim">Kernel</td>
                  <td className="mono">{health.kernel || "—"}</td>
                </tr>
                <tr>
                  <td className="text-dim">Uptime</td>
                  <td>{fmtDuration(health.uptime_seconds)}</td>
                </tr>
                <tr>
                  <td className="text-dim">Processes</td>
                  <td>
                    {health.running_process} running / {health.processes} total
                  </td>
                </tr>
              </tbody>
            </table>
          </div>

          <div className="grid grid-4">
            <StatTile
              label="CPU"
              value={`${health.cpu_percent.toFixed(0)}% of ${health.cpu_count} core${health.cpu_count === 1 ? "" : "s"}`}
              fraction={health.cpu_percent / 100}
            />
            <StatTile
              label="Memory"
              value={`${fmtGB(health.memory_used_gb)} / ${fmtGB(health.memory_total_gb)}`}
              fraction={health.memory_used_pct / 100}
            />
            <StatTile
              label="Disk"
              value={`${fmtGB(health.disk_used_gb)} / ${fmtGB(health.disk_total_gb)}`}
              fraction={health.disk_used_pct / 100}
            />
            <StatTile
              label="Load"
              value={`${health.load_avg_1.toFixed(2)} / ${health.load_avg_5.toFixed(2)} / ${health.load_avg_15.toFixed(2)}`}
              fraction={health.cpu_count > 0 ? health.load_avg_1 / health.cpu_count : 0}
            />
          </div>

          {health.swap_total_gb > 0 && (
            <div className="card">
              <div className="card-title">Swap</div>
              <div className="grid grid-2">
                <StatTile
                  label="Swap used"
                  value={`${fmtGB(health.swap_used_gb)} / ${fmtGB(health.swap_total_gb)}`}
                  fraction={health.swap_total_gb > 0 ? health.swap_used_gb / health.swap_total_gb : 0}
                />
              </div>
            </div>
          )}
        </>
      ) : (
        <div className="card">
          <div className="text-dim">Loading...</div>
        </div>
      )}
    </>
  );
}

function StatTile({ label, value, fraction }: { label: string; value: string; fraction: number }) {
  const cls = fraction > 0.9 ? "danger" : fraction > 0.7 ? "warn" : "";
  return (
    <div className="stat-tile">
      <div className="stat-value" style={{ fontSize: 18 }}>
        {value}
      </div>
      <div className="stat-label">{label}</div>
      <div className="meter">
        <div className={`meter-fill ${cls}`} style={{ width: `${Math.min(Math.max(fraction * 100, 0), 100)}%` }} />
      </div>
    </div>
  );
}