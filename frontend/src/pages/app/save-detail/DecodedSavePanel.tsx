import type { SaveCheatEditorState } from "../../../services/types";
import type { SaveInsightModel } from "../../../utils/saveInsights";
import { buildCheatFactRows, mergeDetailRows } from "./helpers";

type DecodedSavePanelProps = {
  insight: SaveInsightModel | null;
  systemName: string;
  cheatData: SaveCheatEditorState | null;
  cheatLoading: boolean;
  cheatError: string | null;
};

export function DecodedSavePanel({ insight, systemName, cheatData, cheatLoading, cheatError }: DecodedSavePanelProps): JSX.Element {
  const parserGameplayRows = insight?.rows.filter((row) => row.kind === "gameplay") ?? [];
  const cheatRows = buildCheatFactRows(cheatData);
  const gameplayRows = mergeDetailRows(parserGameplayRows, cheatRows).slice(0, 18);
  if (gameplayRows.length > 0) {
    const cheatOnly = parserGameplayRows.length === 0 && cheatRows.length > 0;
    return (
      <section className="save-detail-panel" aria-labelledby="save-detail-decoder-title">
        <div className="save-detail-panel__header">
          <div>
            <p className="save-detail-eyebrow">{cheatOnly ? "Editable save values" : "Decoded gameplay"}</p>
            <h3 id="save-detail-decoder-title">{cheatOnly ? "Cheat-backed save values" : insight?.title || "Gameplay facts"}</h3>
            <p>{cheatOnly ? "Current values read through the same safe parser-backed editor used by cheats." : insight?.subtitle || "Parser-backed facts from the current save."}</p>
          </div>
          <span className="save-detail-tag save-detail-tag--good">{cheatOnly ? cheatData?.editorId || "cheat parser" : insight?.parserId || "parser active"}</span>
        </div>
        <div className="save-detail-gameplay-grid">
          {gameplayRows.map((row) => (
            <div className="save-detail-gameplay-card" key={row.label}>
              <span>{row.label}</span>
              <strong>{row.value}</strong>
            </div>
          ))}
        </div>
        {cheatError ? <p className="error-state">Could not read cheat-backed values: {cheatError}</p> : null}
        {insight?.warnings.length ? (
          <div className="save-detail-note-line">
            {insight.warnings.slice(0, 2).map((warning) => (
              <span key={warning}>{warning}</span>
            ))}
          </div>
        ) : null}
      </section>
    );
  }

  const verifiedRows = insight?.rows.filter((row) => row.kind !== "gameplay").slice(0, 8) ?? [];
  if (verifiedRows.length > 0 || insight) {
    return (
      <section className="save-detail-panel" aria-labelledby="save-detail-decoder-title">
        <div className="save-detail-panel__header">
          <div>
            <p className="save-detail-eyebrow">Verified save facts</p>
            <h3 id="save-detail-decoder-title">{insight?.title || "Save verified"}</h3>
            <p>{cheatLoading ? "Checking parser-backed cheat values..." : insight?.subtitle || "Verified backend metadata from the current save."}</p>
          </div>
          <span className="save-detail-tag">{insight?.parserId || "verified"}</span>
        </div>
        {verifiedRows.length > 0 ? (
          <div className="save-detail-gameplay-grid">
            {verifiedRows.map((row) => (
              <div className="save-detail-gameplay-card save-detail-gameplay-card--verified" key={row.label}>
                <span>{row.label}</span>
                <strong>{row.value}</strong>
              </div>
            ))}
          </div>
        ) : null}
        {cheatError ? <p className="error-state">Could not read cheat-backed values: {cheatError}</p> : null}
        {insight?.warnings.length ? (
          <div className="save-detail-note-line">
            {insight.warnings.slice(0, 2).map((warning) => (
              <span key={warning}>{warning}</span>
            ))}
          </div>
        ) : null}
      </section>
    );
  }

  if (!cheatLoading) {
    return (
      <section className="save-detail-panel save-detail-decoder-empty" aria-labelledby="save-detail-decoder-title">
        <div>
          <p className="save-detail-eyebrow">Save protected</p>
          <h3 id="save-detail-decoder-title">No decoder attached yet</h3>
          <p>
            This save is still protected and versioned. Add a parser-backed Game Support Module for {systemName} to show lives, world, stage, inventory, and other fun details here automatically.
          </p>
        </div>
        <span className="save-detail-tag">Waiting for parser</span>
      </section>
    );
  }

  return (
    <section className="save-detail-panel save-detail-decoder-empty" aria-labelledby="save-detail-decoder-title">
      <div>
        <p className="save-detail-eyebrow">Gameplay decoder</p>
        <h3 id="save-detail-decoder-title">Reading save values...</h3>
        <p>Checking parser-backed cheat values for this save.</p>
      </div>
      <span className="save-detail-tag">Loading</span>
    </section>
  );
}
