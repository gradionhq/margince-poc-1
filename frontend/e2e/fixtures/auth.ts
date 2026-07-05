import { type Page, test as base } from "@playwright/test";

const DEFAULT_EMAIL = process.env.E2E_USER_EMAIL ?? "admin@example.com";
const DEFAULT_PASSWORD = process.env.E2E_USER_PASSWORD ?? "changeme";

export async function login(
  page: Page,
  email = DEFAULT_EMAIL,
  password = DEFAULT_PASSWORD,
): Promise<void> {
  await page.goto("/login");
  await page.getByLabel("Email").fill(email);
  await page.getByLabel("Password").fill(password);
  await page.getByRole("button", { name: "Sign in" }).click();
  await page.waitForURL((url) => !url.pathname.includes("/login"));
}

export const test = base.extend<{ authedPage: Page }>({
  authedPage: async ({ page }, use) => {
    await login(page);
    await use(page);
  },
});

export { expect } from "@playwright/test";
