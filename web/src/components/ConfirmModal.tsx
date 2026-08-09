import { useState } from "react";
import { Modal } from "./Modal";
import { ApiError } from "../api";

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
      <p>{body}</p>
      <div className="modal-actions">
        <button type="button" className="btn" onClick={onClose} disabled={busy}>
          Cancel
        </button>
        <button type="button" className="btn btn-danger" onClick={handleConfirm} disabled={busy}>
          {busy ? "Deleting..." : confirmLabel}
        </button>
      </div>
    </Modal>
  );
}
