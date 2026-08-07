import { useState, type FormEvent } from "react";
import { api, ApiError } from "../api";

export function LoginPage({ mode, onSuccess }: { mode: "setup" | "login"; onSuccess: () => void }) {
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  const submit = async (e: FormEvent) => {
    e.preventDefault();
    setError(null);
    setBusy(true);
    try {
      if (mode === "setup") {
        await api.post("/api/auth/setup", { email, password });
      } else {
        await api.post("/api/auth/login", { email, password });
      }
      onSuccess();
    } catch (e) {
      setError(e instanceof ApiError ? e.message : "Something went wrong");
    } finally {
      setBusy(false);
    }
  };

  return (
    <div className="login-shell">
      <div className="card login-card">
        <h1>{mode === "setup" ? "Welcome to Mangrove" : "Sign in"}</h1>
        <p className="subtitle">
          {mode === "setup"
            ? "Create the admin account to finish setting up this instance."
            : "Sign in to continue to your dashboard."}
        </p>

        {error && <div className="error-banner">{error}</div>}

        <form onSubmit={submit}>
          <div className="field">
            <label htmlFor="email">Email</label>
            <input
              id="email"
              className="input"
              type="email"
              autoComplete="username"
              required
              value={email}
              onChange={(e) => setEmail(e.target.value)}
            />
          </div>
          <div className="field">
            <label htmlFor="password">Password</label>
            <input
              id="password"
              className="input"
              type="password"
              autoComplete={mode === "setup" ? "new-password" : "current-password"}
              required
              minLength={mode === "setup" ? 8 : undefined}
              value={password}
              onChange={(e) => setPassword(e.target.value)}
            />
            {mode === "setup" && <div className="field-hint">At least 8 characters.</div>}
          </div>
          <button className="btn btn-primary" type="submit" disabled={busy} style={{ width: "100%", justifyContent: "center" }}>
            {busy ? "..." : mode === "setup" ? "Create account" : "Sign in"}
          </button>
        </form>
      </div>
    </div>
  );
}
