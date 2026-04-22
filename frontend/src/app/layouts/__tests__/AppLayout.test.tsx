import { render, screen } from "@testing-library/react";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import { afterEach, describe, expect, it, vi } from "vitest";
import { AppLayout } from "../AppLayout";

vi.mock("../../../services/authSession", () => ({
  clearFrontendAuthSession: vi.fn(),
  isFrontendAuthRequired: vi.fn(() => false)
}));

describe("AppLayout", () => {
  afterEach(() => {
    vi.clearAllMocks();
  });

  it("shows My Saves in the nav and hides obsolete entries", () => {
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
    expect(screen.queryByRole("link", { name: "My Games" })).not.toBeInTheDocument();
    expect(screen.queryByRole("link", { name: "Getting Started" })).not.toBeInTheDocument();
    expect(screen.queryByRole("link", { name: "Download" })).not.toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Devices" })).toBeInTheDocument();
    expect(screen.getByText("My Saves content")).toBeInTheDocument();
  });
});
