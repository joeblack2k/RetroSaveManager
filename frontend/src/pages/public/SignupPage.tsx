import { FormEvent, useState } from "react";
import { SectionCard } from "../../components/SectionCard";
import { signup } from "../../services/retrosaveApi";

export function SignupPage(): JSX.Element {
  const [email, setEmail] = useState("internal@local");
  const [password, setPassword] = useState("internal");
  const [result, setResult] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);

  async function onSubmit(event: FormEvent<HTMLFormElement>): Promise<void> {
    event.preventDefault();
    setError(null);
    setResult(null);
    try {
      const response = await signup(email, password);
      setResult(response.message || "Signup successful");
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : "Signup mislukt");
    }
  }

  return (
    <SectionCard title="Signup" subtitle="Beschikbaar voor compatibiliteit; afdwingende auth is uitgeschakeld.">
      <form className="stack" onSubmit={onSubmit}>
        <label className="field">
          <span>E-mail</span>
          <input value={email} onChange={(event) => setEmail(event.target.value)} required type="email" />
        </label>
        <label className="field">
          <span>Password</span>
          <input value={password} onChange={(event) => setPassword(event.target.value)} required type="password" />
        </label>
        <button className="btn btn-primary" type="submit">
          Account aanmaken
        </button>
      </form>
      {result ? <p className="success-state">{result}</p> : null}
      {error ? <p className="error-state">{error}</p> : null}
    </SectionCard>
  );
}
