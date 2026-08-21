import { useState, type FormEvent } from "react";
import { api, ApiError } from "../api";

export function SettingsPage() {
  return (
    <>
      <div className="page-header">
        <div>
          <h1>Settings</h1>
          <p>Manage your account.</p>
        </div>
      </div>

      <div className="card">
        <div className="card-title">Change password</div>
        <ChangePasswordForm />
      </div>
    </>
  );
}

function ChangePasswordForm() {
  const [currentPassword, setCurrentPassword] = useState("");
  const [newPassword, setNewPassword] = useState("");
  const [confirmPassword, setConfirmPassword] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [success, setSuccess] = useState(false);

  const mismatch = confirmPassword.length > 0 && newPassword !== confirmPassword;
  const canSubmit = currentPassword.length > 0 && newPassword.length >= 8 && newPassword === confirmPassword;

  const submit = async (e: FormEvent) => {
    e.preventDefault();
    if (!canSubmit) return;
    setBusy(true);
    setError(null);
    setSuccess(false);
    try {
      await api.post("/api/auth/change-password", { current_password: currentPassword, new_password: newPassword });
      setCurrentPassword("");
      setNewPassword("");
      setConfirmPassword("");
      setSuccess(true);
    } catch (e) {
      setError(e instanceof ApiError ? e.message : "Failed to change password");
    } finally {
      setBusy(false);
    }
  };

  return (
    <form onSubmit={submit} style={{ maxWidth: 360, marginTop: 14 }}>
      {error && <div className="error-banner">{error}</div>}
      {success && <div className="field-hint">Password updated.</div>}

      <div className="field">
        <label htmlFor="current-password">Current password</label>
        <input
          id="current-password"
          className="input mono"
          type="password"
          autoComplete="current-password"
          value={currentPassword}
          onChange={(e) => setCurrentPassword(e.target.value)}
        />
      </div>

      <div className="field">
        <label htmlFor="new-password">New password</label>
        <input
          id="new-password"
          className="input mono"
          type="password"
          autoComplete="new-password"
          value={newPassword}
          onChange={(e) => setNewPassword(e.target.value)}
          placeholder="min. 8 characters"
        />
      </div>

      <div className="field">
        <label htmlFor="confirm-password">Confirm new password</label>
        <input
          id="confirm-password"
          className="input mono"
          type="password"
          autoComplete="new-password"
          value={confirmPassword}
          onChange={(e) => setConfirmPassword(e.target.value)}
        />
        {mismatch && <div className="field-hint">Passwords don't match.</div>}
      </div>

      <button className="btn btn-primary btn-sm" type="submit" disabled={busy || !canSubmit}>
        Update password
      </button>
    </form>
  );
}
