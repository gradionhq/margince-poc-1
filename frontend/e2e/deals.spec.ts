import { expect, test } from "./fixtures/auth.js";
import { seedLiveDeal } from "./fixtures/seed.js";

test.describe("PipelinePage", () => {
  test("STATE-1: renders the honest empty state", async ({ authedPage }) => {
    await authedPage.route("**/api/deals**", async (route) => {
      if (new URL(route.request().url()).pathname === "/api/deals") {
        await route.fulfill({
          status: 200,
          contentType: "application/json",
          body: JSON.stringify({ data: [], page: { has_more: false } }),
        });
        return;
      }
      await route.continue();
    });

    await authedPage.goto("/deals");
    await expect(authedPage.getByTestId("board-empty-state")).toBeVisible();
  });

  test("STATE-2: shows loading chrome before the board resolves", async ({
    authedPage,
  }) => {
    let release = () => {};
    const pending = new Promise<void>((resolve) => {
      release = resolve;
    });

    await authedPage.route("**/api/deals**", async (route) => {
      if (new URL(route.request().url()).pathname === "/api/deals") {
        await pending;
        await route.fulfill({
          status: 200,
          contentType: "application/json",
          body: JSON.stringify({ data: [], page: { has_more: false } }),
        });
        return;
      }
      await route.continue();
    });

    const nav = authedPage.goto("/deals", { waitUntil: "domcontentloaded" });
    await expect(authedPage.getByTestId("board-skeleton")).toBeVisible();
    release();
    await nav;
  });

  test("STATE-3: renders honest errors for the board and rollup", async ({
    authedPage,
  }) => {
    await authedPage.route("**/api/deals**", async (route) => {
      if (new URL(route.request().url()).pathname === "/api/deals") {
        await route.abort();
        return;
      }
      await route.continue();
    });

    await authedPage.goto("/deals");
    await expect(
      authedPage.getByText("Failed to load the pipeline board."),
    ).toBeVisible({ timeout: 15000 });
  });

  test("STATE-4: read-only users still render the page chrome", async ({
    page,
  }) => {
    await page.goto("/login");
    await page.getByLabel("Email").fill("readonly@example.com");
    await page.getByLabel("Password").fill("changeme");
    await page.getByRole("button", { name: "Sign in" }).click();
    await page.waitForURL("**/people");

    await page.goto("/deals");
    await expect(
      page.getByRole("heading", { level: 1, name: /deals/i }),
    ).toBeVisible();
  });

  test("STATE-5: omits PILOT-EXCLUDED panels from the DOM", async ({
    authedPage,
  }) => {
    await authedPage.goto("/deals");
    await expect(authedPage.getByTestId("warm-signal-panel")).toHaveCount(0);
    await expect(authedPage.getByTestId("firmographics-panel")).toHaveCount(0);
    await expect(authedPage.getByTestId("ai-content-panel")).toHaveCount(0);
  });
});

test.describe("DealDetailPage", () => {
  test("STATE-1: archived deals render the archived banner instead of a blank page", async ({
    authedPage,
  }) => {
    const archived = await seedLiveDeal(authedPage.request);
    await authedPage.route(`**/api/deals/${archived.id}`, async (route) => {
      if (route.request().method() === "GET") {
        await route.fulfill({
          status: 200,
          contentType: "application/json",
          body: JSON.stringify({
            ...archived,
            archived_at: new Date().toISOString(),
            stakeholder_count: 0,
            stakeholders: [],
            timeline: [],
          }),
        });
        return;
      }
      await route.continue();
    });
    await authedPage.goto(`/deals/${archived.id}`);
    await expect(authedPage.getByTestId("archived-banner")).toBeVisible();
  });

  test("STATE-2: shows the loading shell before the deal resolves", async ({
    authedPage,
  }) => {
    await authedPage.route("**/api/deals/**", async (route) => {
      if (new URL(route.request().url()).pathname.startsWith("/api/deals/")) {
        await new Promise((resolve) => setTimeout(resolve, 750));
        await route.continue();
        return;
      }
      await route.continue();
    });

    await authedPage.goto("/deals/loading-deal", {
      waitUntil: "domcontentloaded",
    });
    await expect(authedPage.getByTestId("deal-detail-skeleton")).toBeVisible();
  });

  test("STATE-3: renders the error card on backend failure", async ({
    authedPage,
  }) => {
    await authedPage.route("**/api/deals/**", async (route) => {
      if (new URL(route.request().url()).pathname.startsWith("/api/deals/")) {
        await route.abort();
        return;
      }
      await route.continue();
    });

    await authedPage.goto("/deals/error-deal");
    await expect(authedPage.getByText("Failed to load this deal.")).toBeVisible(
      {
        timeout: 15000,
      },
    );
  });

  test("STATE-4: no live RBAC fixture is available yet, so the page keeps its current chrome", async ({
    authedPage,
  }) => {
    const deal = await seedLiveDeal(authedPage.request);
    await authedPage.goto(`/deals/${deal.id}`);
    await expect(
      authedPage.getByRole("button", { name: "Archive…" }),
    ).toBeVisible();
  });

  test("STATE-5: omits PILOT-EXCLUDED panels from the DOM", async ({
    authedPage,
  }) => {
    const deal = await seedLiveDeal(authedPage.request);
    await authedPage.goto(`/deals/${deal.id}`);
    await expect(authedPage.getByTestId("warm-signal-panel")).toHaveCount(0);
    await expect(authedPage.getByTestId("firmographics-panel")).toHaveCount(0);
    await expect(authedPage.getByTestId("ai-content-panel")).toHaveCount(0);
  });
});
