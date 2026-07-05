import { expect, test } from "./fixtures/auth.js";
import { seedArchivedPerson } from "./fixtures/seed.js";

test.describe("PeoplePage", () => {
  test("STATE-1: renders the honest empty state", async ({ authedPage }) => {
    await authedPage.route("**/api/people**", async (route) => {
      if (new URL(route.request().url()).pathname === "/api/people") {
        await route.fulfill({
          status: 200,
          contentType: "application/json",
          body: JSON.stringify({ data: [], page: { has_more: false } }),
        });
        return;
      }
      await route.continue();
    });

    await authedPage.goto("/people");
    await expect(authedPage.getByText("No contacts yet.")).toBeVisible();
  });

  test("STATE-2: shows loading chrome before the list resolves", async ({
    authedPage,
  }) => {
    let release = () => {};
    const pending = new Promise<void>((resolve) => {
      release = resolve;
    });

    await authedPage.route("**/api/people**", async (route) => {
      if (new URL(route.request().url()).pathname === "/api/people") {
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

    const nav = authedPage.goto("/people", { waitUntil: "domcontentloaded" });
    await expect(authedPage.getByTestId("person-list-skeleton")).toBeVisible();
    release();
    await nav;
  });

  test("STATE-3: renders an honest error card on backend failure", async ({
    authedPage,
  }) => {
    await authedPage.route("**/api/people**", async (route) => {
      if (new URL(route.request().url()).pathname === "/api/people") {
        await route.abort();
        return;
      }
      await route.continue();
    });

    await authedPage.goto("/people");
    await expect(authedPage.getByTestId("person-list-error")).toBeVisible({
      timeout: 15000,
    });
    await expect(authedPage.getByRole("button", { name: "Retry" })).toBeVisible({
      timeout: 15000,
    });
  });

  test("STATE-4: read-only users still render the page chrome and the existing field gate", async ({
    page,
  }) => {
    await page.goto("/login");
    await page.getByLabel("Email").fill("readonly@example.com");
    await page.getByLabel("Password").fill("changeme");
    await page.getByRole("button", { name: "Sign in" }).click();
    await page.waitForURL("**/people");

    await expect(page.getByText(/contacts we actually know/i)).toBeVisible();
    await expect(page.getByTestId("field-guard-masked")).toBeVisible();
  });

  test("STATE-5: omits PILOT-EXCLUDED panels from the DOM", async ({
    authedPage,
  }) => {
    await authedPage.goto("/people");
    await expect(authedPage.getByTestId("warm-signal-panel")).toHaveCount(0);
    await expect(authedPage.getByTestId("firmographics-panel")).toHaveCount(0);
    await expect(authedPage.getByTestId("ai-content-panel")).toHaveCount(0);
  });
});

test.describe("PersonDetailPage", () => {
  test("STATE-1: archived people render the archived banner instead of a blank page", async ({
    authedPage,
  }) => {
    const archived = await seedArchivedPerson(authedPage.request);
    await authedPage.goto(`/people/${archived.id}`);
    await expect(authedPage.getByTestId("archived-banner")).toBeVisible();
    await expect(authedPage.getByRole("button", { name: "Restore" })).toBeVisible();
  });

  test("STATE-2: shows the loading shell before the person resolves", async ({
    authedPage,
  }) => {
    await authedPage.route("**/api/people/**", async (route) => {
      if (new URL(route.request().url()).pathname.startsWith("/api/people/")) {
        await new Promise((resolve) => setTimeout(resolve, 750));
        await route.continue();
        return;
      }
      await route.continue();
    });

    await authedPage.goto("/people/loading-person", {
      waitUntil: "domcontentloaded",
    });
    await expect(authedPage.getByTestId("person-detail-loading")).toBeVisible();
  });

  test("STATE-3: renders the error card on backend failure", async ({
    authedPage,
  }) => {
    await authedPage.route("**/api/people/**", async (route) => {
      if (new URL(route.request().url()).pathname.startsWith("/api/people/")) {
        await route.abort();
        return;
      }
      await route.continue();
    });

    await authedPage.goto("/people/error-person");
    await expect(authedPage.getByTestId("person-detail-error")).toBeVisible({
      timeout: 15000,
    });
  });

  test("STATE-5: omits PILOT-EXCLUDED panels from the DOM", async ({
    authedPage,
  }) => {
    const archived = await seedArchivedPerson(authedPage.request);
    await authedPage.goto(`/people/${archived.id}`);
    await expect(authedPage.getByTestId("warm-signal-panel")).toHaveCount(0);
    await expect(authedPage.getByTestId("firmographics-panel")).toHaveCount(0);
    await expect(authedPage.getByTestId("ai-content-panel")).toHaveCount(0);
  });
});
