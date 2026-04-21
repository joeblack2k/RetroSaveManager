export function LoadingState({ label = "Laden..." }: { label?: string }): JSX.Element {
  return (
    <div className="load-state" role="status" aria-live="polite">
      <span className="load-state__dot" />
      <span>{label}</span>
    </div>
  );
}

export function ErrorState({ message }: { message: string }): JSX.Element {
  return <p className="error-state">{message}</p>;
}
