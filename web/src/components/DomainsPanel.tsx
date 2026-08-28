import { useEffect, useState } from "react";
import { api, ApiError, type CustomDomain } from "../api";
import { useIsOwner } from "../userContext";

function errMsg(e: unknown): string {
  return e instanceof ApiError ? e.message : e instanceof Error ? e.message : String(e);
}

// DomainsPanel lets an owner point a real domain at this deployment --
// Mangrove programs a host-matched Caddy route once the domain's DNS TXT
// record proves ownership, and Caddy provisions/renews the TLS
// certificate on its own from there (see internal/proxy/caddy.go's
// PutDomainRoute and internal/orchestrator/domains.go).
export function DomainsPanel({ deploymentId }: { deploymentId: number }) {
  const isOwner = useIsOwner();
  const [domains, setDomains] = useState<CustomDomain[] | null>(null);
  const [hostname, setHostname] = useState("");
  const [adding, setAdding] = useState(false);
  const [verifyingId, setVerifyingId] = useState<number | null>(null);
  const [removingId, setRemovingId] = useState<number | null>(null);
  const [error, setError] = useState<string | null>(null);

  const load = () => {
    api
      .get<CustomDomain[]>(`/api/deployments/${deploymentId}/domains`)
      .then((d) => setDomains(d ?? []))
      .catch((e) => setError(errMsg(e)));
  };
  useEffect(load, [deploymentId]);

  const add = async () => {
    if (!hostname.trim()) return;
    setAdding(true);
    setError(null);
    try {
      await api.post(`/api/deployments/${deploymentId}/domains`, { hostname: hostname.trim() });
      setHostname("");
      load();
    } catch (e) {
      setError(errMsg(e));
    } finally {
      setAdding(false);
    }
  };

  const verify = async (id: number) => {
    setVerifyingId(id);
    setError(null);
    try {
      await api.post(`/api/domains/${id}/verify`);
      load();
    } catch (e) {
      setError(errMsg(e));
    } finally {
      setVerifyingId(null);
    }
  };

  const remove = async (id: number) => {
    setRemovingId(id);
    setError(null);
    try {
      await api.del(`/api/domains/${id}`);
      load();
    } catch (e) {
      setError(errMsg(e));
    } finally {
      setRemovingId(null);
    }
  };

  return (
    <div className="card">
      <div className="card-title">Domains</div>
      <p className="text-dim" style={{ marginTop: 0 }}>
        Point a custom domain at this deployment. Mangrove terminates HTTPS for it automatically once verified.
      </p>
      {error && <div className="error-banner">{error}</div>}

      {domains && domains.length > 0 && (
        <table style={{ marginBottom: 16 }}>
          <thead>
            <tr>
              <th>Hostname</th>
              <th>Status</th>
              {isOwner && <th />}
            </tr>
          </thead>
          <tbody>
            {domains.map((d) => (
              <tr key={d.id}>
                <td>{d.hostname}</td>
                <td>
                  {d.verified ? (
                    <span className="pill pill-green">
                      <span className="pill-dot" />
                      Verified
                    </span>
                  ) : (
                    <div>
                      <span className="pill pill-yellow">
                        <span className="pill-dot" />
                        Pending verification
                      </span>
                      <div className="field-hint" style={{ marginTop: 4 }}>
                        Add this TXT record on <code>{d.hostname}</code>, then verify:
                        <br />
                        <code>mangrove-domain-verification={d.verification_token}</code>
                      </div>
                      {isOwner && (
                        <button className="btn btn-sm" style={{ marginTop: 6 }} onClick={() => verify(d.id)} disabled={verifyingId === d.id}>
                          {verifyingId === d.id ? "Verifying..." : "Verify"}
                        </button>
                      )}
                    </div>
                  )}
                </td>
                {isOwner && (
                  <td>
                    <button className="btn btn-sm btn-danger" onClick={() => remove(d.id)} disabled={removingId === d.id}>
                      {removingId === d.id ? "Removing..." : "Remove"}
                    </button>
                  </td>
                )}
              </tr>
            ))}
          </tbody>
        </table>
      )}
      {domains && domains.length === 0 && <div className="empty-state">No custom domains yet.</div>}

      {isOwner && (
        <div className="field" style={{ display: "flex", gap: 8, alignItems: "flex-end" }}>
          <div style={{ flex: 1 }}>
            <label htmlFor="add-domain-hostname">Add a domain</label>
            <input
              id="add-domain-hostname"
              className="input"
              placeholder="app.example.com"
              value={hostname}
              onChange={(e) => setHostname(e.target.value)}
            />
          </div>
          <button className="btn btn-sm" onClick={add} disabled={adding || !hostname.trim()}>
            {adding ? "Adding..." : "Add"}
          </button>
        </div>
      )}
    </div>
  );
}
