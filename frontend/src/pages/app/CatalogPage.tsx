import { useCallback } from "react";
import { SectionCard } from "../../components/SectionCard";
import { ErrorState, LoadingState } from "../../components/LoadState";
import { useAsyncData } from "../../hooks/useAsyncData";
import { addLibraryGame, listCatalog, listLibrary, removeLibraryGame } from "../../services/retrosaveApi";

export function CatalogPage(): JSX.Element {
  const loader = useCallback(async () => {
    const [catalog, library] = await Promise.all([listCatalog(), listLibrary()]);
    return { catalog, library };
  }, []);

  const { loading, error, data, reload } = useAsyncData(loader, []);

  async function addToLibrary(catalogId: string): Promise<void> {
    await addLibraryGame(catalogId);
    reload();
  }

  async function removeFromLibrary(libraryId: string): Promise<void> {
    await removeLibraryGame(libraryId);
    reload();
  }

  return (
    <div className="grid two-cols">
      <SectionCard title="Catalog" subtitle="Beschikbare games uit catalog endpoints.">
        {loading ? <LoadingState label="Catalog laden..." /> : null}
        {error ? <ErrorState message={error} /> : null}
        {data ? (
          <ul className="plain-list">
            {data.catalog.map((item) => (
              <li key={item.id} className="list-row">
                <div>
                  <strong>{item.name}</strong>
                  <p>{item.description}</p>
                </div>
                <button className="btn btn-ghost" type="button" onClick={() => void addToLibrary(item.id)}>
                  Add to library
                </button>
              </li>
            ))}
          </ul>
        ) : null}
      </SectionCard>

      <SectionCard title="Library" subtitle="Jouw huidige library entries.">
        {loading ? <LoadingState label="Library laden..." /> : null}
        {error ? <ErrorState message={error} /> : null}
        {data ? (
          <ul className="plain-list">
            {data.library.map((entry) => (
              <li key={entry.id} className="list-row">
                <div>
                  <strong>{entry.catalog.name}</strong>
                  <p>{entry.catalog.system.name}</p>
                </div>
                <button className="btn btn-ghost" type="button" onClick={() => void removeFromLibrary(entry.id)}>
                  Remove
                </button>
              </li>
            ))}
          </ul>
        ) : null}
      </SectionCard>
    </div>
  );
}
