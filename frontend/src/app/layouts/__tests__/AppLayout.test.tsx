import { act, fireEvent, render, screen } from "@testing-library/react";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { AppLayout } from "../AppLayout";
import { enableAutoAppPasswordEnrollment, getAutoAppPasswordEnrollmentStatus } from "../../../services/retrosaveApi";

vi.mock("../../../services/authSession", () => ({
  clearFrontendAuthSession: vi.fn(),
  isFrontendAuthRequired: vi.fn(() => false)
}));

vi.mock("../../../services/retrosaveApi", () => ({
  enableAutoAppPasswordEnrollment: vi.fn(),
  getAutoAppPasswordEnrollmentStatus: vi.fn(() => Promise.resolve({ active: false, enabledUntil: null }))
}));

describe("AppLayout", () => {
  beforeEach(() => {
    vi.useRealTimers();
    vi.mocked(getAutoAppPasswordEnrollmentStatus).mockResolvedValue({
      active: false,
      enabledUntil: null
    });
    vi.mocked(enableAutoAppPasswordEnrollment).mockResolvedValue({
      active: true,
      enabledUntil: "2026-04-23T12:15:00Z"
    });
  });

  afterEach(() => {
    vi.clearAllMocks();
    vi.useRealTimers();
  });

  it("shows My Saves in the nav and hides obsolete entries", async () => {
    render(
      <MemoryRouter initialEntries={["/app/my-games"]}>
        <Routes>
          <Route path="/app" element={<AppLayout />}>
            <Route path="my-games" element={<div>My Saves content</div>} />
          </Route>
        </Routes>
      </MemoryRouter>
    );

    expect(screen.getByRole("link", { name: "My Saves" })).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Cheats" })).toBeInTheDocument();
    expect(screen.queryByRole("link", { name: "My Games" })).not.toBeInTheDocument();
    expect(screen.queryByRole("link", { name: "Getting Started" })).not.toBeInTheDocument();
    expect(screen.queryByRole("link", { name: "Download" })).not.toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Validation" })).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Logs" })).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Devices" })).toBeInTheDocument();
    expect(screen.getByText("My Saves content")).toBeInTheDocument();
    expect(await screen.findByRole("button", { name: "Add helper" })).toBeInTheDocument();
  });

  it("replaces Add helper with a live countdown timer after activation", async () => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date("2026-04-23T12:00:00Z"));

    render(
      <MemoryRouter initialEntries={["/app/my-games"]}>
        <Routes>
          <Route path="/app" element={<AppLayout />}>
            <Route path="my-games" element={<div>My Saves content</div>} />
          </Route>
        </Routes>
      </MemoryRouter>
    );

    await act(async () => {
      await Promise.resolve();
    });

    const button = screen.getByRole("button", { name: "Add helper" });

    await act(async () => {
      fireEvent.click(button);
      await Promise.resolve();
    });

    expect(screen.getByText("15:00")).toBeInTheDocument();

    act(() => {
      vi.advanceTimersByTime(1000);
    });

    expect(screen.getByText("14:59")).toBeInTheDocument();
  });
});
