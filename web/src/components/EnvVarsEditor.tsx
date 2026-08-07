import { useEffect, useState, type FormEvent } from "react";
import { api, ApiError, type EnvVarEntry } from "../api";

export function EnvVarsEditor({ serviceId }: { serviceId: number }) {
  const [vars, setVars] = useState<EnvVarEntry[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [key, setKey] = useState("");
  const [value, setValue] = useState("");
  const [isSecret, setIsSecret] = useState(false);
  const [busy, setBusy] = useState(false);

  const load = () => {
    api
      .get<EnvVarEntry[]>(`/api/services/${serviceId}/env`)
      .then((v) => setVars(v ?? []))
      .catch((e) => setError(e instanceof ApiError ? e.message : "Failed to load env vars"));
  };

  useEffect(load, [serviceId]);

  const add = async (e: FormEvent) => {
    e.preventDefault();
    if (!key) return;
    setBusy(true);
    setError(null);
    try {
      await api.put(`/api/services/${serviceId}/env/${encodeURIComponent(key)}`, { value, is_secret: isSecret });
      setKey("");
      setValue("");
      setIsSecret(false);
      load();
    } catch (e) {
      setError(e instanceof ApiError ? e.message : "Failed to save");
    } finally {
      setBusy(false);
    }
  };

  const remove = async (k: string) => {
    await api.del(`/api/services/${serviceId}/env/${encodeURIComponent(k)}`);
    load();
  };

  return (
    <div>
      {error && <div className="error-banner">{error}</div>}
      {vars && vars.length > 0 && (
        <div className="kv-list" style={{ marginBottom: 16 }}>
          {vars.map((v) => (
            <div className="kv-row" key={v.key}>
              <span className="kv-key">{v.key}</span>
              <span className="kv-value text-dim">{v.is_secret ? "•••••••• (encrypted)" : v.value}</span>
              <button className="btn btn-sm btn-danger" onClick={() => remove(v.key)}>
                Remove
              </button>
            </div>
          ))}
        </div>
      )}
      <form onSubmit={add} className="form-row" style={{ alignItems: "flex-end" }}>
        <div className="field" style={{ marginBottom: 0 }}>
          <label htmlFor={`env-key-${serviceId}`}>Key</label>
          <input
            id={`env-key-${serviceId}`}
            className="input mono"
            value={key}
            onChange={(e) => setKey(e.target.value.toUpperCase())}
            placeholder="API_KEY"
          />
        </div>
        <div className="field" style={{ marginBottom: 0 }}>
          <label htmlFor={`env-value-${serviceId}`}>Value</label>
          <input
            id={`env-value-${serviceId}`}
            className="input mono"
            value={value}
            onChange={(e) => setValue(e.target.value)}
            type={isSecret ? "password" : "text"}
          />
        </div>
        <div className="field" style={{ marginBottom: 0, flex: "0 0 auto" }}>
          <label style={{ display: "flex", alignItems: "center", gap: 6, whiteSpace: "nowrap" }}>
            <input type="checkbox" checked={isSecret} onChange={(e) => setIsSecret(e.target.checked)} />
            Secret
          </label>
        </div>
        <button className="btn" type="submit" disabled={busy || !key}>
          Add
        </button>
      </form>
    </div>
  );
}
