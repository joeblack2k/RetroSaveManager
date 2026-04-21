import { FormEvent, useState } from "react";
import { Link, useNavigate, useSearchParams } from "react-router-dom";
import { SectionCard } from "../../components/SectionCard";
import { login } from "../../services/retrosaveApi";
import { isFrontendAuthRequired, markFrontendAuthSession } from "../../services/authSession";

export function LoginPage(): JSX.Element {
  const [email, setEmail] = useState("internal@local");
  const [password, setPassword] = useState("internal");
  const [message, setMessage] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);
  const navigate = useNavigate();
  const [params] = useSearchParams();
  const authRequired = isFrontendAuthRequired();

  async function onSubmit(event: FormEvent<HTMLFormElement>): Promise<void> {
    event.preventDefault();
    setLoading(true);
    setMessage(null);
    setError(null);
    try {
      const result = await login(email, password);
      setMessage(result.message || "Login gelukt");
      if (isFrontendAuthRequired()) {
        markFrontendAuthSession();
      }
      const requestedNext = (params.get("next") ?? "").trim();
      const nextPath = requestedNext.startsWith("/") ? requestedNext : "/app/my-games";
      navigate(nextPath, { replace: true });
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : "Login mislukt");
    } finally {
      setLoading(false);
    }
  }

  return (
    <SectionCard
      title="Login"
      subtitle={
        authRequired
          ? "Auth mode is actief: log in om naar je dashboard te gaan."
          : "Auth mode staat uit: dashboard blijft direct beschikbaar."
      }
    >
      <form className="stack" onSubmit={onSubmit}>
        <label className="field">
          <span>E-mail</span>
          <input value={email} onChange={(event) => setEmail(event.target.value)} required type="email" />
        </label>
        <label className="field">
          <span>Password</span>
          <input value={password} onChange={(event) => setPassword(event.target.value)} required type="password" />
        </label>
        <div className="inline-actions">
          <button className="btn btn-primary" type="submit" disabled={loading}>
            {loading ? "Bezig..." : "Login"}
          </button>
          <Link className="btn btn-ghost" to="/forgot-password">
            Wachtwoord vergeten
          </Link>
          <Link className="btn btn-ghost" to="/signup">
            Signup
          </Link>
        </div>
      </form>
      {message ? <p className="success-state">{message}</p> : null}
      {error ? <p className="error-state">{error}</p> : null}
    </SectionCard>
  );
}
