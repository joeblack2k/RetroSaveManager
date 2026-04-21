import { FormEvent, useState } from "react";
import { SectionCard } from "../../components/SectionCard";
import { resetPassword } from "../../services/retrosaveApi";

export function ResetPasswordPage(): JSX.Element {
  const [token, setToken] = useState("internal-token");
  const [password, setPassword] = useState("new-password");
  const [message, setMessage] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);

  async function onSubmit(event: FormEvent<HTMLFormElement>): Promise<void> {
    event.preventDefault();
    setMessage(null);
    setError(null);
    try {
      const response = await resetPassword(token, password);
      setMessage(response.message || "Password reset verwerkt");
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : "Reset mislukt");
    }
  }

  return (
    <SectionCard title="Reset password" subtitle="Compatibele endpoint-flow voor clients en web UI.">
      <form className="stack" onSubmit={onSubmit}>
        <label className="field">
          <span>Token</span>
          <input value={token} onChange={(event) => setToken(event.target.value)} required />
        </label>
        <label className="field">
          <span>Nieuw password</span>
          <input value={password} onChange={(event) => setPassword(event.target.value)} required type="password" />
        </label>
        <button className="btn btn-primary" type="submit">
          Reset uitvoeren
        </button>
      </form>
      {message ? <p className="success-state">{message}</p> : null}
      {error ? <p className="error-state">{error}</p> : null}
    </SectionCard>
  );
}
