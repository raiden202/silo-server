import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, it, expect, vi } from "vitest";
import RecipeCard from "./RecipeCard";

describe("RecipeCard", () => {
  it("renders preset name, icon, description, and category tag", () => {
    render(
      <RecipeCard
        preset={{
          key: "k",
          display_name: "Hidden Gems",
          icon: "💎",
          description_short: "Underrated picks",
          default_params: {},
        }}
        category="discovery"
        onPick={() => {}}
      />,
    );
    expect(screen.getByText("Hidden Gems")).toBeInTheDocument();
    expect(screen.getByText("💎")).toBeInTheDocument();
    expect(screen.getByText("Underrated picks")).toBeInTheDocument();
    expect(screen.getByText(/discovery/i)).toBeInTheDocument();
  });

  it("calls onPick when clicked", async () => {
    const onPick = vi.fn();
    render(
      <RecipeCard
        preset={{
          key: "k",
          display_name: "X",
          icon: "X",
          description_short: "X",
          default_params: {},
        }}
        category="discovery"
        onPick={onPick}
      />,
    );
    await userEvent.click(screen.getByRole("button", { name: /X/ }));
    expect(onPick).toHaveBeenCalled();
  });
});
