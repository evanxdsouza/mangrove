import { test, expect, type Page, type BrowserContext } from "@playwright/test";

// Each test file in this suite runs against a real Mangrove instance + real
// local Docker (started by run-e2e.sh with a fresh SQLite DB), never a
// mocked API -- matching the rest of this project's "test against the real
// thing" approach.
//
// All tests below share a single browser context/page (created once in
// beforeAll, closed in afterAll) instead of using Playwright's default
// fresh-context-per-test fixture -- these tests are intentionally
// stateful (login session, a deployment created in one test and deployed
// in the next), and a fresh context per test would silently drop the
// session cookie between them.

const FIXTURE_DIR = process.env.MANGROVE_FIXTURE_DIR;
if (!FIXTURE_DIR) {
  throw new Error("MANGROVE_FIXTURE_DIR env var must point at the nginx Dockerfile fixture repo");
}
const FIXTURE_GIT_URL = `file://${FIXTURE_DIR}`;

let context: BrowserContext;
let page: Page;
let deploymentId: string;

test.beforeAll(async ({ browser }) => {
  context = await browser.newContext();
  page = await context.newPage();
});

test.afterAll(async () => {
  await context.close();
});

test.describe.serial("Mangrove dashboard", () => {
  test("first-run admin setup", async () => {
    await page.goto("/");
    await expect(page.getByText("Welcome to Mangrove")).toBeVisible();
    await page.getByLabel("Email").fill("evan@example.com");
    await page.getByLabel("Password").fill("correcthorsebatterystaple");
    await page.getByRole("button", { name: "Create account" }).click();
    await expect(page.getByRole("heading", { name: "Projects" })).toBeVisible();
  });

  test("login rate limit locks out after repeated failures", async () => {
    // Pure API check against the login endpoint -- doesn't touch the
    // page's own session, so it can't disturb the logged-in state the
    // later tests depend on.
    let lastStatus = 0;
    for (let i = 0; i < 6; i++) {
      const resp = await page.request.post("/api/auth/login", {
        data: { email: "evan@example.com", password: "wrong-password" },
      });
      lastStatus = resp.status();
    }
    expect(lastStatus).toBe(429);

    // The shared page should still be logged in from the previous test --
    // confirm the lockout attempts above didn't clobber that session.
    await page.reload();
    await expect(page.getByRole("heading", { name: "Projects" })).toBeVisible();
  });

  test("create project, create deployment, deploy, and see it running", async () => {
    await page.goto("/");
    await page.getByRole("button", { name: "+ New project" }).click();
    await page.getByLabel("Name", { exact: true }).fill("E2E Test Project");
    await page.getByRole("button", { name: "Create project" }).click();
    await expect(page.getByRole("heading", { name: "E2E Test Project" })).toBeVisible();

    await page.getByRole("button", { name: "+ New deployment" }).click();
    await page.getByLabel("Name", { exact: true }).fill("web");
    await page.getByLabel("Branch").fill("main");
    await page.getByLabel("Internal port").fill("80");
    await page.getByLabel("Health check path").fill("/");
    // The create-deployment form doesn't take a git URL directly (that's
    // supplied at deploy time via the API/CLI, matching how a webhook-driven
    // deploy would work) -- create the deployment, then trigger the first
    // deploy via the API with the fixture's file:// URL, same as the CLI
    // smoke tests earlier in this build.
    await page.getByRole("button", { name: "Create deployment" }).click();
    await expect(page).toHaveURL(/\/deployments\/\d+$/);

    deploymentId = page.url().split("/deployments/")[1];
    const deployResp = await page.request.post(`/api/deployments/${deploymentId}/deploy`, {
      data: { git_url: FIXTURE_GIT_URL, git_ref: "main" },
    });
    expect(deployResp.ok()).toBeTruthy();

    await page.reload();
    await expect(page.getByText("running", { exact: false }).first()).toBeVisible({ timeout: 30_000 });

    await page.getByText("History").click();
    await expect(page.getByText("success").first()).toBeVisible();
  });

  test("redeploy and roll back via the timeline", async () => {
    expect(deploymentId).toBeTruthy();

    const redeployResp = await page.request.post(`/api/deployments/${deploymentId}/deploy`, {
      data: { git_url: FIXTURE_GIT_URL, git_ref: "main" },
    });
    expect(redeployResp.ok()).toBeTruthy();

    await page.reload();
    await page.getByText("History").click();
    const rows = page.locator(".timeline-item");
    await expect(rows).toHaveCount(2);

    // Revert to the older (first) deploy -- it's the second item in the
    // newest-first timeline and is no longer "current", so it should have
    // a revert button.
    await rows.nth(1).getByRole("button", { name: "Revert to this" }).click();
    await expect(rows).toHaveCount(3, { timeout: 30_000 });
    await expect(page.getByText("rollback of #").first()).toBeVisible();
  });

  test("toggling password protection enforces it at the proxy layer", async () => {
    expect(deploymentId).toBeTruthy();

    await page.goto(`/projects/1/deployments/${deploymentId}`);
    await page.getByLabel("Public (expose on the assigned port)").check();
    await page.getByLabel("Password-protected").check();
    await page.getByLabel("Password", { exact: true }).fill("supersecretpassword123");
    await page.getByRole("button", { name: "Save" }).click();
    await expect(page.getByText("Saved.")).toBeVisible();

    // Read back the assigned host port from the service card, then hit it
    // directly (bypassing the dashboard's own session) to confirm Caddy
    // itself is enforcing basic auth -- not just the API.
    await page.reload();
    const hostPortText = await page.locator(".kv-row", { hasText: "Host port" }).locator(".kv-value").innerText();
    const hostPort = hostPortText.trim();
    expect(hostPort).toMatch(/^\d+$/);

    const unauthed = await page.request.get(`http://127.0.0.1:${hostPort}/`, { failOnStatusCode: false });
    expect(unauthed.status()).toBe(401);

    const authed = await page.request.get(`http://127.0.0.1:${hostPort}/`, {
      failOnStatusCode: false,
      headers: { Authorization: "Basic " + Buffer.from("mangrove:supersecretpassword123").toString("base64") },
    });
    expect(authed.status()).toBe(200);
  });

  test("admin panel shows resource budget and the running node", async () => {
    await page.goto("/admin");
    await expect(page.getByText("Resource budget")).toBeVisible();
    await expect(page.getByText(/\d+ \/ \d+ MB/)).toBeVisible();
    await expect(page.getByText("local").first()).toBeVisible();
  });
});
