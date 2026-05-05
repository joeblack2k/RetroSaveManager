import { AlertTriangle } from "lucide-react";

type ConfirmDialogProps = {
  title: string;
  message: string;
  confirmLabel: string;
  cancelLabel?: string;
  danger?: boolean;
  busy?: boolean;
  onConfirm: () => void;
  onCancel: () => void;
};

export function ConfirmDialog({
  title,
  message,
  confirmLabel,
  cancelLabel = "Cancel",
  danger = false,
  busy = false,
  onConfirm,
  onCancel
}: ConfirmDialogProps): JSX.Element {
  return (
    <div className="treegrid-modal-backdrop" role="presentation" onClick={busy ? undefined : onCancel}>
      <section
        className="confirm-dialog"
        role="dialog"
        aria-modal="true"
        aria-labelledby="confirm-dialog-title"
        onClick={(event) => event.stopPropagation()}
      >
        <header className="confirm-dialog__header">
          <AlertTriangle aria-hidden="true" />
          <div>
            <h2 id="confirm-dialog-title">{title}</h2>
            <p>{message}</p>
          </div>
        </header>
        <footer className="confirm-dialog__actions">
          <button className="treegrid-select-button treegrid-select-button--ghost" type="button" onClick={onCancel} disabled={busy}>
            {cancelLabel}
          </button>
          <button
            className={danger ? "treegrid-select-button treegrid-select-button--danger" : "treegrid-select-button"}
            type="button"
            onClick={onConfirm}
            disabled={busy}
          >
            {busy ? "Working..." : confirmLabel}
          </button>
        </footer>
      </section>
    </div>
  );
}
