import { expect, test } from "./fixtures/auth.js";
import { seedArchivedOrganization } from "./fixtures/seed.js";

test.describe("CompaniesPage", () => {
  test("STATE-1: renders the honest empty state", async ({ authedPage }) => {
    await authedPage.route("**/api/organizations**", async (route) => {
      if (new URL(route.request().url()).pathname === "/api/organizations") {
        await route.fulfill({
          status: 200,
          contentType: "application/json",
          body: JSON.stringify({ data: [], page: { has_more: false } }),
        });
        return;
      }
      await route.continue();
    });

    await authedPage.goto("/companies");
    await expect(authedPage.getByText("No companies yet.")).toBeVisible();
  });

  test("STATE-2: shows loading chrome before the list resolves", async ({
    authedPage,
  }) => {
    let release = () => {};
    const pending = new Promise<void>((resolve) => {
      release = resolve;
    });

    await authedPage.route("**/api/organizations**", async (route) => {
      if (new URL(route.request().url()).pathname === "/api/organizations") {
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

    const nav = authedPage.goto("/companies", { waitUntil: "domcontentloaded" });
    await expect(authedPage.getByTestId("company-list-skeleton")).toBeVisible();
    release();
    await nav;
  });

  test("STATE-3: renders an honest error card on backend failure", async ({
    authedPage,
  }) => {
    await authedPage.route("**/api/organizations**", async (route) => {
      if (new URL(route.request().url()).pathname === "/api/organizations") {
        await route.abort();
        return;
      }
      await route.continue();
    });

    await authedPage.goto("/companies");
    await expect(authedPage.getByText("Failed to load companies.")).toBeVisible({
      timeout: 15000,
    });
    await expect(authedPage.getByRole("button", { name: "Retry" })).toBeVisible({
      timeout: 15000,
    });
  });

  test("STATE-4: read-only users still render the page chrome", async ({
    page,
  }) => {
    await page.goto("/login");
    await page.getByLabel("Email").fill("readonly@example.com");
    await page.getByLabel("Password").fill("changeme");
    await page.getByRole("button", { name: "Sign in" }).click();
    await page.waitForURL("**/people");

    await page.goto("/companies");
    await expect(page.getByRole("heading", { name: /companies/i })).toBeVisible();
  });

  test("STATE-5: omits PILOT-EXCLUDED panels from the DOM", async ({
    authedPage,
  }) => {
    await authedPage.goto("/companies");
    await expect(authedPage.getByTestId("warm-signal-panel")).toHaveCount(0);
    await expect(authedPage.getByTestId("firmographics-panel")).toHaveCount(0);
    await expect(authedPage.getByTestId("ai-content-panel")).toHaveCount(0);
  });
});

test.describe("CompanyDetailPage", () => {
  test("STATE-1: archived companies render the archived banner instead of a blank page", async ({
    authedPage,
  }) => {
    const archived = await seedArchivedOrganization(authedPage.request);
    await authedPage.goto(`/companies/${archived.id}`);
    await expect(authedPage.getByTestId("archived-banner")).toBeVisible();
  });

  test("STATE-2: shows the loading shell before the company resolves", async ({
    authedPage,
  }) => {
    await authedPage.route("**/api/organizations/**", async (route) => {
      if (new URL(route.request().url()).pathname.startsWith("/api/organizations/")) {
        await new Promise((resolve) => setTimeout(resolve, 750));
        await route.continue();
        return;
      }
      await route.continue();
    });

    await authedPage.goto("/companies/loading-company", {
      waitUntil: "domcontentloaded",
    });
    await expect(authedPage.getByTestId("company-detail-skeleton")).toBeVisible();
  });

  test("STATE-3: renders the error card on backend failure", async ({
    authedPage,
  }) => {
    await authedPage.route("**/api/organizations/**", async (route) => {
      if (new URL(route.request().url()).pathname.startsWith("/api/organizations/")) {
        await route.abort();
        return;
      }
      await route.continue();
    });

    await authedPage.goto("/companies/error-company");
    await expect(
      authedPage.getByText("Failed to load this company."),
    ).toBeVisible({ timeout: 15000 });
  });

  test("STATE-4: no live RBAC fixture is available yet, so the page keeps its current chrome", async ({
    authedPage,
  }) => {
    const archived = await seedArchivedOrganization(authedPage.request);
    await authedPage.goto(`/companies/${archived.id}`);
    await expect(authedPage.getByRole("button", { name: "Edit" })).toBeVisible();
  });

  test("STATE-5: omits PILOT-EXCLUDED panels from the DOM", async ({
    authedPage,
  }) => {
    const archived = await seedArchivedOrganization(authedPage.request);
    await authedPage.goto(`/companies/${archived.id}`);
    await expect(authedPage.getByTestId("warm-signal-panel")).toHaveCount(0);
    await expect(authedPage.getByTestId("firmographics-panel")).toHaveCount(0);
    await expect(authedPage.getByTestId("ai-content-panel")).toHaveCount(0);
  });
});
