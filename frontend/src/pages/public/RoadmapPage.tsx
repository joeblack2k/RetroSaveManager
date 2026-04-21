import { FormEvent, useCallback, useState } from "react";
import { SectionCard } from "../../components/SectionCard";
import { ErrorState, LoadingState } from "../../components/LoadState";
import { useAsyncData } from "../../hooks/useAsyncData";
import { createRoadmapSuggestion, listMyRoadmapSuggestions, listRoadmapItems, voteRoadmapItem } from "../../services/retrosaveApi";
import { formatDate } from "../../utils/format";

export function RoadmapPage(): JSX.Element {
  const loader = useCallback(async () => {
    const [items, mine] = await Promise.all([listRoadmapItems(), listMyRoadmapSuggestions()]);
    return { items, mine };
  }, []);

  const { loading, error, data, reload } = useAsyncData(loader, []);
  const [title, setTitle] = useState("");
  const [description, setDescription] = useState("");
  const [submitError, setSubmitError] = useState<string | null>(null);

  async function handleVote(id: string): Promise<void> {
    await voteRoadmapItem(id);
    reload();
  }

  async function handleSubmit(event: FormEvent<HTMLFormElement>): Promise<void> {
    event.preventDefault();
    setSubmitError(null);
    try {
      await createRoadmapSuggestion(title, description);
      setTitle("");
      setDescription("");
      reload();
    } catch (err: unknown) {
      setSubmitError(err instanceof Error ? err.message : "Kon suggestie niet opslaan");
    }
  }

  return (
    <div className="grid two-cols">
      <SectionCard title="Roadmap" subtitle="Stem op bestaande punten of voeg nieuwe suggesties toe.">
        {loading ? <LoadingState label="Roadmap laden..." /> : null}
        {error ? <ErrorState message={error} /> : null}
        {data ? (
          <ul className="plain-list">
            {data.items.map((item) => (
              <li key={item.id} className="list-row">
                <div>
                  <strong>{item.title}</strong>
                  <p>{item.description}</p>
                  <small>{formatDate(item.createdAt)}</small>
                </div>
                <button className="btn btn-ghost" type="button" onClick={() => void handleVote(item.id)}>
                  Vote ({item.votes})
                </button>
              </li>
            ))}
          </ul>
        ) : null}
      </SectionCard>

      <SectionCard title="Suggestie indienen" subtitle="Jouw suggesties onderaan zichtbaar.">
        <form className="stack" onSubmit={handleSubmit}>
          <label className="field">
            <span>Titel</span>
            <input value={title} onChange={(event) => setTitle(event.target.value)} required />
          </label>
          <label className="field">
            <span>Beschrijving</span>
            <textarea rows={4} value={description} onChange={(event) => setDescription(event.target.value)} />
          </label>
          <button className="btn btn-primary" type="submit">
            Suggestie opslaan
          </button>
        </form>
        {submitError ? <ErrorState message={submitError} /> : null}
        {data ? (
          <div>
            <h3>Mijn suggesties</h3>
            <ul className="plain-list">
              {data.mine.map((item) => (
                <li key={item.id}>
                  <strong>{item.title}</strong>
                  <p>{item.description}</p>
                </li>
              ))}
            </ul>
          </div>
        ) : null}
      </SectionCard>
    </div>
  );
}
