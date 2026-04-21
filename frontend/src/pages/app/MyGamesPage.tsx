import { ChangeEvent, useCallback, useMemo, useState } from "react";
import { ErrorState, LoadingState } from "../../components/LoadState";
import { useAsyncData } from "../../hooks/useAsyncData";
import { apiDownloadURL } from "../../services/apiClient";
import { getCurrentUser, listSaves } from "../../services/retrosaveApi";
import { formatBytes, formatRelativeDate } from "../../utils/format";

type SystemOption = {
  slug: string;
  name: string;
};

type SaveRow = {
  key: string;
  gameName: string;
  systemName: string;
  systemSlug: string;
  saveCount: number;
  totalBytes: number;
  latestCreatedAt: string;
  latestVersion: number;
  coverUrl: string;
  downloadUrl: string;
};

const DEFAULT_LIMIT_BYTES = 200 * 1024 * 1024;

export function MyGamesPage(): JSX.Element {
  const [systemFilter, setSystemFilter] = useState("all");

  const loader = useCallback(async () => {
    const [user, saves] = await Promise.all([getCurrentUser(), listSaves()]);
    return { user, saves };
  }, []);

  const { loading, error, data } = useAsyncData(loader, []);

  const systemOptions = useMemo<SystemOption[]>(() => {
    if (!data) {
      return [];
    }
    const map = new Map<string, string>();
    for (const save of data.saves) {
      const slug = save.game.system?.slug?.trim() || "unknown";
      const name = save.game.system?.name?.trim() || "Unknown system";
      if (!map.has(slug)) {
        map.set(slug, name);
      }
    }
    return [...map.entries()]
      .map(([slug, name]) => ({ slug, name }))
      .sort((a, b) => a.name.localeCompare(b.name));
  }, [data]);

  const rows = useMemo<SaveRow[]>(() => {
    if (!data) {
      return [];
    }
    const grouped = new Map<string, SaveRow & { saveIDs: string[] }>();
    const filtered = data.saves.filter((save) => {
      if (systemFilter === "all") {
        return true;
      }
      const slug = save.game.system?.slug?.trim() || "unknown";
      return slug === systemFilter;
    });

    for (const save of filtered) {
      const systemSlug = save.game.system?.slug?.trim() || "unknown";
      const systemName = save.game.system?.name?.trim() || "Unknown system";
      const gameName = save.game.name?.trim() || "Unknown game";
      const key = `${systemSlug}:${save.game.id}:${gameName}`;
      const createdAt = save.createdAt || new Date(0).toISOString();
      const explicitCover = save.game.boxartThumb || save.game.boxart;

      const existing = grouped.get(key);
      if (!existing) {
        grouped.set(key, {
          key,
          gameName,
          systemName,
          systemSlug,
          saveCount: 1,
          totalBytes: save.fileSize,
          latestCreatedAt: createdAt,
          latestVersion: save.version,
          coverUrl: explicitCover || buildFallbackCover(gameName),
          saveIDs: [save.id],
          downloadUrl: ""
        });
        continue;
      }

      existing.saveCount += 1;
      existing.totalBytes += save.fileSize;
      existing.latestVersion = Math.max(existing.latestVersion, save.version);
      if (new Date(createdAt).getTime() > new Date(existing.latestCreatedAt).getTime()) {
        existing.latestCreatedAt = createdAt;
      }
      existing.saveIDs.push(save.id);
      if (!explicitCover && existing.coverUrl) {
        continue;
      }
      if (explicitCover) {
        existing.coverUrl = explicitCover;
      }
    }

    const list = [...grouped.values()].map((row) => ({
      key: row.key,
      gameName: row.gameName,
      systemName: row.systemName,
      systemSlug: row.systemSlug,
      saveCount: row.saveCount,
      totalBytes: row.totalBytes,
      latestCreatedAt: row.latestCreatedAt,
      latestVersion: row.latestVersion,
      coverUrl: row.coverUrl,
      downloadUrl: row.saveIDs.length === 1
        ? apiDownloadURL(`/saves/download?id=${encodeURIComponent(row.saveIDs[0])}`)
        : apiDownloadURL(`/saves/download_many?ids=${encodeURIComponent(row.saveIDs.join(","))}`)
    }));

    list.sort((a, b) => new Date(b.latestCreatedAt).getTime() - new Date(a.latestCreatedAt).getTime());
    return list;
  }, [data, systemFilter]);

  const totalSaveCount = useMemo(() => rows.reduce((sum, row) => sum + row.saveCount, 0), [rows]);
  const totalBytes = useMemo(() => rows.reduce((sum, row) => sum + row.totalBytes, 0), [rows]);
  const usagePercent = Math.min(100, (totalBytes / DEFAULT_LIMIT_BYTES) * 100);

  function handleSystemFilterChange(event: ChangeEvent<HTMLSelectElement>): void {
    setSystemFilter(event.target.value);
  }

  if (loading) {
    return <LoadingState label="My Saves laden..." />;
  }

  if (error) {
    return <ErrorState message={error} />;
  }

  return (
    <section className="saves-board fade-in-up">
      <header className="saves-board__header">
        <div>
          <h2>My Saves</h2>
          <p>
            {rows.length} games ({totalSaveCount} saves) · {formatBytes(totalBytes)} / {formatBytes(DEFAULT_LIMIT_BYTES)}{" "}
            <span className="saves-plan-tag">Free</span>
          </p>
          <div className="saves-board__progress" aria-label="Storage usage">
            <span style={{ width: `${usagePercent}%` }} />
          </div>
        </div>
        <div className="saves-board__actions">
          <select className="saves-system-select" value={systemFilter} onChange={handleSystemFilterChange} aria-label="System filter" name="systemFilter">
            <option value="all">All systems</option>
            {systemOptions.map((option) => (
              <option key={option.slug} value={option.slug}>
                {option.name}
              </option>
            ))}
          </select>
          <button className="saves-toolbar-btn" type="button">
            Upload
          </button>
          <button className="saves-toolbar-icon" type="button" aria-label="List view">
            ≡
          </button>
          <button className="saves-toolbar-icon" type="button" aria-label="Grid view">
            ▦
          </button>
        </div>
      </header>

      <div className="saves-table-wrap">
        <table className="saves-table">
          <thead>
            <tr>
              <th className="saves-table__check">
                <input type="checkbox" aria-label="Select all rows" name="selectAllSaves" />
              </th>
              <th>Game</th>
              <th>#</th>
              <th>Size</th>
              <th>Version</th>
              <th>Date</th>
              <th>Download</th>
            </tr>
          </thead>
          <tbody>
            {rows.map((row) => (
              <tr key={row.key}>
                <td className="saves-table__check">
                  <input type="checkbox" aria-label={`Select ${row.gameName}`} name={`select-${row.key}`} />
                </td>
                <td>
                  <div className="saves-game-cell">
                    <img src={row.coverUrl} alt={`${row.gameName} cover`} className="saves-cover" loading="lazy" />
                    <div>
                      <strong>{row.gameName}</strong>
                      <p>
                        {row.systemName} · {row.saveCount} save{row.saveCount === 1 ? "" : "s"}
                      </p>
                    </div>
                  </div>
                </td>
                <td>{row.saveCount}</td>
                <td>{formatBytes(row.totalBytes)}</td>
                <td>v{row.latestVersion}</td>
                <td>{formatRelativeDate(row.latestCreatedAt)}</td>
                <td>
                  <a className="saves-download-btn" href={row.downloadUrl}>
                    Download
                  </a>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </section>
  );
}

function buildFallbackCover(title: string): string {
  const initials = title
    .split(/\s+/)
    .slice(0, 2)
    .map((part) => part.slice(0, 1).toUpperCase())
    .join("") || "NA";
  const safeText = initials.replace(/[^A-Z0-9]/g, "");

  const svg = `
<svg xmlns="http://www.w3.org/2000/svg" width="128" height="128" viewBox="0 0 128 128">
  <defs>
    <linearGradient id="g" x1="0" y1="0" x2="1" y2="1">
      <stop offset="0%" stop-color="#4B5563" />
      <stop offset="100%" stop-color="#1F2937" />
    </linearGradient>
  </defs>
  <rect width="128" height="128" rx="12" fill="url(#g)" />
  <rect x="10" y="10" width="108" height="108" rx="8" fill="none" stroke="#9CA3AF" stroke-width="2" />
  <text x="64" y="72" text-anchor="middle" fill="#E5E7EB" font-family="Arial, sans-serif" font-size="34" font-weight="700">${safeText}</text>
</svg>`;
  return `data:image/svg+xml;charset=UTF-8,${encodeURIComponent(svg)}`;
}
