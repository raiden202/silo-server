import type { ReactNode } from "react";
import { MemoryRouter } from "react-router";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { renderToStaticMarkup } from "react-dom/server";
import { beforeEach, describe, expect, it, vi } from "vitest";

const mocks = vi.hoisted(() => ({
  useQuery: vi.fn(),
}));

vi.mock("@tanstack/react-query", async () => {
  const actual =
    await vi.importActual<typeof import("@tanstack/react-query")>("@tanstack/react-query");
  return {
    ...actual,
    useQuery: (...args: unknown[]) => mocks.useQuery(...args),
  };
});

vi.mock("@/hooks/useDebounce", () => ({
  useDebounce: <T,>(v: T) => v,
}));

vi.mock("@/components/ui/dialog", () => ({
  Dialog: ({ children, open }: { children: ReactNode; open: boolean }) =>
    open ? <div data-testid="dialog">{children}</div> : null,
  DialogContent: ({ children }: { children: ReactNode }) => <div>{children}</div>,
  DialogTitle: ({ children }: { children: ReactNode }) => <h2>{children}</h2>,
}));

vi.mock("@/lib/thumbhash", () => ({
  decodeThumbhash: () => "",
}));

import { GlobalSearch } from "./GlobalSearch";

const browseFixture = {
  content_id: "movie-99",
  type: "movie" as const,
  title: "Test Movie",
  year: 2020,
  genres: [] as string[],
  content_rating: "PG",
  status: "matched" as const,
  rating_imdb: null as number | null,
  overview: "",
  poster_url: "",
  poster_thumbhash: "",
  backdrop_url: "",
  backdrop_thumbhash: "",
};

function renderSearchMarkup(props: Partial<Parameters<typeof GlobalSearch>[0]> = {}) {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return renderToStaticMarkup(
    <QueryClientProvider client={client}>
      <MemoryRouter>
        <GlobalSearch {...props} />
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

describe("GlobalSearch", () => {
  beforeEach(() => {
    mocks.useQuery.mockReset();
    mocks.useQuery.mockReturnValue({
      data: {
        total: 50,
        has_more: true,
        items: [browseFixture],
      },
      isFetching: false,
      isError: false,
    });
  });

  it("renders preview rows and total hint when more results exist than the preview limit", () => {
    const markup = renderSearchMarkup({ defaultOpen: true, initialQuery: "Test" });

    expect(markup).toContain('data-testid="dialog"');
    expect(markup).toContain("Test Movie");
    expect(markup).toContain("Showing top 8 of 50");
    expect(markup).toContain("Press Enter for all results");
  });

  it("disables the preview query when the dialog is closed", () => {
    const client = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    });
    renderToStaticMarkup(
      <QueryClientProvider client={client}>
        <MemoryRouter>
          <GlobalSearch />
        </MemoryRouter>
      </QueryClientProvider>,
    );

    expect(mocks.useQuery).toHaveBeenCalled();
    const lastCall = mocks.useQuery.mock.calls[mocks.useQuery.mock.calls.length - 1]![0] as {
      enabled: boolean;
    };
    expect(lastCall.enabled).toBe(false);
  });
});
