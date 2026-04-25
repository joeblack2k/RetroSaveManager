import { FormEvent, useState } from "react";
import { useParams } from "react-router-dom";
import { SectionCard } from "../../components/SectionCard";
import { verifyDevice } from "../../services/retrosaveApi";

export function DeviceVerifyPage(): JSX.Element {
  const params = useParams<{ code: string }>();
  const [code, setCode] = useState((params.code ?? "").toUpperCase());
  const [message, setMessage] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);

  async function onSubmit(event: FormEvent<HTMLFormElement>): Promise<void> {
    event.preventDefault();
    setMessage(null);
    setError(null);
    try {
      const response = await verifyDevice(code);
      const expires = response.expiresAt ? ` (expires ${new Date(response.expiresAt).toLocaleString("en-US")})` : "";
      setMessage(`Device code verified${expires}`);
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : "Verification failed");
    }
  }

  return (
    <SectionCard title="Device verification" subtitle="Use this page for helper device-code flows.">
      <form className="stack" onSubmit={onSubmit}>
        <label className="field">
          <span>Code</span>
          <input value={code} onChange={(event) => setCode(event.target.value.toUpperCase())} required />
        </label>
        <button className="btn btn-primary" type="submit">
          Verify device
        </button>
      </form>
      {message ? <p className="success-state">{message}</p> : null}
      {error ? <p className="error-state">{error}</p> : null}
    </SectionCard>
  );
}
