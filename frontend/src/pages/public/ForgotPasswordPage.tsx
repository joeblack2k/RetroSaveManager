import { FormEvent, useState } from "react";
import { SectionCard } from "../../components/SectionCard";
import { forgotPassword } from "../../services/retrosaveApi";

export function ForgotPasswordPage(): JSX.Element {
  const [email, setEmail] = useState("internal@local");
  const [message, setMessage] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);

  async function onSubmit(event: FormEvent<HTMLFormElement>): Promise<void> {
    event.preventDefault();
    setMessage(null);
    setError(null);
    try {
      const response = await forgotPassword(email);
      setMessage(response.message || "Reset gestart");
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : "Request mislukt");
    }
  }

  return (
    <SectionCard title="Forgot password" subtitle="Flow is aanwezig voor helper/web parity in interne trusted mode.">
      <form className="stack" onSubmit={onSubmit}>
        <label className="field">
          <span>E-mail</span>
          <input value={email} onChange={(event) => setEmail(event.target.value)} required type="email" />
        </label>
        <button className="btn btn-primary" type="submit">
          Verstuur reset link
        </button>
      </form>
      {message ? <p className="success-state">{message}</p> : null}
      {error ? <p className="error-state">{error}</p> : null}
    </SectionCard>
  );
}
