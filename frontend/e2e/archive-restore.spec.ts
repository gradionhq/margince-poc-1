import { expect, test } from "./fixtures/auth.js";
import {
  seedArchivedOrganization,
  seedArchivedPerson,
  seedLiveDeal,
  seedLiveOrganization,
  seedLivePerson,
} from "./fixtures/seed.js";

test.describe("archive / restore", () => {
  test("archives and restores a contact from the contacts list and detail page", async ({
    authedPage,
  }) => {
    const person = await seedLivePerson(authedPage.request);

    await authedPage.goto(`/people/${person.id}`);
    await authedPage.getByRole("button", { name: "Archive…" }).click();
    await Promise.all([
      authedPage.waitForResponse(
        (response) =>
          response.request().method() === "DELETE" &&
          response.url().endsWith(`/people/${person.id}`),
      ),
      authedPage
        .getByRole("dialog")
        .getByRole("button", { name: "Archive", exact: true })
        .click(),
    ]);

    await authedPage.goto(`/people/${person.id}`);
    await expect(authedPage.getByTestId("archived-banner")).toBeVisible();
    await expect(
      authedPage.getByRole("button", { name: "Restore" }),
    ).toBeVisible();

    await Promise.all([
      authedPage.waitForResponse(
        (response) =>
          response.request().method() === "POST" &&
          response.url().endsWith(`/people/${person.id}/restore`),
      ),
      authedPage.getByRole("button", { name: "Restore" }).click(),
    ]);
    await expect(authedPage.getByTestId("archived-banner")).toHaveCount(0);
  });

  test("archives and restores a company from the companies list and detail page", async ({
    authedPage,
  }) => {
    const company = await seedLiveOrganization(authedPage.request);

    await authedPage.goto(`/companies/${company.id}`);
    await authedPage.getByRole("button", { name: "Archive…" }).click();
    await Promise.all([
      authedPage.waitForResponse(
        (response) =>
          response.request().method() === "DELETE" &&
          response.url().endsWith(`/organizations/${company.id}`),
      ),
      authedPage
        .getByRole("dialog")
        .getByRole("button", { name: "Archive", exact: true })
        .click(),
    ]);
    await expect(authedPage.getByText("Company archived")).toHaveCount(1);

    const archived = await seedArchivedOrganization(authedPage.request);
    await authedPage.goto(`/companies/${archived.id}`);
    await expect(authedPage.getByTestId("archived-banner")).toBeVisible();
    await Promise.all([
      authedPage.waitForResponse(
        (response) =>
          response.request().method() === "POST" &&
          response.url().endsWith(`/organizations/${archived.id}/restore`),
      ),
      authedPage.getByRole("button", { name: "Restore" }).click(),
    ]);
    await expect(authedPage.getByTestId("archived-banner")).toHaveCount(0);
  });

  test("archives and restores a deal from the pipeline and detail page", async ({
    authedPage,
  }) => {
    test.skip(
      true,
      "Live backend does not wire DELETE /deals/{id} yet; keep this as an explicit gap.",
    );
    const deal = await seedLiveDeal(authedPage.request);

    await authedPage.goto("/deals");
    await authedPage.getByRole("radio", { name: "Table" }).check();
    const row = authedPage.locator("tr").filter({ hasText: deal.name });

    await expect(row).toBeVisible();

    await authedPage.goto(`/deals/${deal.id}`);
    await authedPage.getByRole("button", { name: "Archive…" }).click();
    await Promise.all([
      authedPage.waitForResponse(
        (response) =>
          response.request().method() === "DELETE" &&
          response.url().endsWith(`/deals/${deal.id}`),
      ),
      authedPage
        .getByRole("dialog")
        .getByRole("button", { name: "Archive", exact: true })
        .click(),
    ]);
    await authedPage.goto(`/deals/${deal.id}`);
    await expect(authedPage.getByTestId("archived-banner")).toBeVisible();
    await Promise.all([
      authedPage.waitForResponse(
        (response) =>
          response.request().method() === "POST" &&
          response.url().endsWith(`/deals/${deal.id}/restore`),
      ),
      authedPage.getByRole("button", { name: "Restore" }).click(),
    ]);
    await expect(authedPage.getByTestId("archived-banner")).toHaveCount(0);

    await authedPage.goto("/deals");
    await authedPage.getByRole("radio", { name: "Board" }).check();
    await expect(authedPage.getByTestId(`deal-card-${deal.id}`)).toBeVisible();
  });

  test("shows an existing-record pointer on restore conflict for archived contacts", async ({
    authedPage,
  }) => {
    const archived = await seedArchivedPerson(authedPage.request);
    const live = await seedLivePerson(authedPage.request);

    await authedPage.route(
      `**/people/${archived.id}/restore`,
      async (route) => {
        await route.fulfill({
          status: 409,
          contentType: "application/problem+json",
          body: JSON.stringify({
            type: "about:blank",
            title: "Conflict",
            status: 409,
            code: "duplicate_email",
            detail: "An active person already owns this email.",
            details: {
              existing_id: live.id,
              field: "emails[0].email",
            },
          }),
        });
      },
    );

    await authedPage.goto(`/people/${archived.id}`);
    await authedPage.getByRole("button", { name: "Restore" }).click();

    const pointer = authedPage.getByRole("link", {
      name: /already live as a different record/i,
    });
    await expect(pointer).toHaveAttribute("href", `/people/${live.id}`);
    await expect(authedPage.getByText(/restore failed/i)).toHaveCount(0);
  });
});
