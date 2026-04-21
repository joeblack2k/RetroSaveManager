import { FormEvent, useState } from "react";
import { SectionCard } from "../../components/SectionCard";
import { apiFetchJSON } from "../../services/apiClient";

export function VerifyEmailPage(): JSX.Element {
  const [token, setToken] = useState("internal-token");
  const [message, setMessage] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);

  async function onSubmit(event: FormEvent<HTMLFormElement>): Promise<void> {
    event.preventDefault();
    setMessage(null);
    setError(null);
    try {
      const response = await apiFetchJSON<{ message?: string }>("/auth/verify-email", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ token })
      });
      setMessage(response.message || "E-mail geverifieerd");
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : "Verificatie mislukt");
    }
  }

  return (
    <SectionCard title="Verify email" subtitle="Stub-compatible endpoint voor web parity.">
      <form className="stack" onSubmit={onSubmit}>
        <label className="field">
          <span>Verification token</span>
          <input value={token} onChange={(event) => setToken(event.target.value)} required />
        </label>
        <button className="btn btn-primary" type="submit">
          Verify e-mail
        </button>
      </form>
      {message ? <p className="success-state">{message}</p> : null}
      {error ? <p className="error-state">{error}</p> : null}
    </SectionCard>
  );
}
