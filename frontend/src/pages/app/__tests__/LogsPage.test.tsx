import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { LogsPage } from "../LogsPage";
import * as retrosaveApi from "../../../services/retrosaveApi";

vi.mock("../../../services/retrosaveApi", () => ({
  listSyncLogs: vi.fn()
}));

describe("LogsPage", () => {
  beforeEach(() => {
    vi.mocked(retrosaveApi.listSyncLogs)
      .mockResolvedValueOnce({
        hours: 72,
        page: 1,
        limit: 50,
        total: 51,
        totalPages: 2,
        logs: [
          {
            id: "log-1",
            createdAt: "2026-04-23T10:00:00Z",
            deviceName: "MiSTer",
            action: "upload",
            game: "Sonic 3",
            error: false
          }
        ]
      })
      .mockResolvedValueOnce({
        hours: 72,
        page: 2,
        limit: 50,
        total: 51,
        totalPages: 2,
        logs: [
          {
            id: "log-2",
            createdAt: "2026-04-23T09:00:00Z",
            deviceName: "RetroArch",
            action: "conflict",
            game: "Tekken 3",
            error: true,
            errorMessage: "Conflict detected"
          }
        ]
      });
  });

  afterEach(() => {
    vi.clearAllMocks();
  });

  it("renders the logs table and paginates", async () => {
    render(
      <MemoryRouter>
        <LogsPage />
      </MemoryRouter>
    );

    expect(await screen.findByRole("heading", { name: "Logs" })).toBeInTheDocument();
    expect(screen.getByText("MiSTer")).toBeInTheDocument();
    expect(screen.getByText("Sonic 3")).toBeInTheDocument();
    expect(screen.getByText("No")).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Next" }));

    await waitFor(() => {
      expect(retrosaveApi.listSyncLogs).toHaveBeenLastCalledWith({ hours: 72, page: 2, limit: 50 });
    });
    expect(await screen.findByText("RetroArch")).toBeInTheDocument();
    expect(screen.getByText("Conflict")).toBeInTheDocument();
    expect(screen.getByText("Yes")).toBeInTheDocument();
    expect(screen.getByText("Conflict detected")).toBeInTheDocument();
  });
});
