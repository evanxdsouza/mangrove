import { useState } from "react";
import { Modal } from "./Modal";
import { ApiError } from "../api";
import { AlertIcon, TrashIcon } from "../icons";

/**
 * Confirmation modal for destructive actions (delete project/deployment).
 * Unlike the bare-onClick deletes in AdminPage (PATs/ports/sessions), these
 * tear down live containers and Caddy routes, so they get an explicit
 * confirm step rather than firing on the first click.
 */
export function ConfirmModal({
  title,
  body,
  confirmLabel,
  onConfirm,
  onClose,
}: {
  title: string;
  body: string;
  confirmLabel: string;
  onConfirm: () => Promise<void>;
  onClose: () => void;
}) {
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const handleConfirm = async () => {
    setBusy(true);
    setError(null);
    try {
      await onConfirm();
    } catch (e) {
      setError(e instanceof ApiError ? e.message : "Something went wrong");
      setBusy(false);
    }
  };

  return (
    <Modal title={title} onClose={onClose}>
      {error && <div className="error-banner">{error}</div>}
      <p style={{ display: "flex", gap: 10, alignItems: "flex-start" }}>
        <AlertIcon style={{ width: 17, height: 17, color: "var(--red-bright)", flexShrink: 0, marginTop: 1 }} />
        <span>{body}</span>
      </p>
      <div className="modal-actions">
        <button type="button" className="btn" onClick={onClose} disabled={busy}>
          Cancel
        </button>
        <button type="button" className="btn btn-danger" onClick={handleConfirm} disabled={busy}>
          <TrashIcon /> {busy ? "Deleting..." : confirmLabel}
        </button>
      </div>
    </Modal>
  );
}
