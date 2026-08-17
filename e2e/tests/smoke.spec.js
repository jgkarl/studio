// End-to-end walk through the app's golden path, module by module, against a live server
// (Docker container running the real Debian/libvips build — see e2e/README.md). Serial: each
// step builds on state the previous one created (client -> asset -> treatment -> project ->
// report), same as a real conservator's session would. Screenshots land in docs/screenshots/,
// numbered in the order a first-time user would actually see these pages.
const { test, expect } = require("@playwright/test");
const path = require("path");

const SCREENSHOT_DIR = path.join(__dirname, "..", "..", "docs", "screenshots");
const TEST_PHOTO = path.join(__dirname, "..", "fixtures", "test-photo.png");
// The dev-login picker shows each user's name + role (not email — see internal/auth/views.templ's
// devLoginList), while Settings shows the email in its inlined Users table — set both via
// BOOTSTRAP_ADMIN_NAME/BOOTSTRAP_ADMIN_EMAIL when starting the server for this suite.
const ADMIN_NAME = process.env.E2E_ADMIN_NAME || "Ada Admin";
const ADMIN_EMAIL = process.env.E2E_ADMIN_EMAIL || "ada@studio.local";

async function shoot(page, name) {
  await page.screenshot({ path: path.join(SCREENSHOT_DIR, name), fullPage: true });
}

test.describe.configure({ mode: "serial" });

let clientId, assetId, treatmentId, projectId, reportId;

// A single shared page/context across every test in this file (rather than each test's own
// `page` fixture, which starts a fresh, unauthenticated context) — the whole point of this suite
// is one continuous session through the app, logging in once and carrying that session's cookie
// through client -> asset -> treatment -> project -> report.
let page;

test.beforeAll(async ({ browser }) => {
  const context = await browser.newContext();
  page = await context.newPage();
});

test.afterAll(async () => {
  await page.close();
});

test.describe("Studio golden path", () => {
  test("dev login as the bootstrapped admin", async () => {
    await page.goto("/login");
    await expect(page.locator("h1")).toContainText("Studio");
    await shoot(page, "01-login.png");

    // Dev login picker lists every existing user by name — click the bootstrapped admin's row.
    await page.click(`form[action="/login/dev"]:has-text("${ADMIN_NAME}") button[type="submit"]`);
    await expect(page).toHaveURL("/");
    await shoot(page, "02-dashboard.png");
  });

  test("Settings is one flat screen with the seeded classifier groups", async () => {
    await page.goto("/settings");
    await expect(page.getByRole("heading", { name: "Asset Types" })).toBeVisible();
    await expect(page.getByRole("heading", { name: "Treatment Methods" })).toBeVisible();
    await expect(page.getByRole("heading", { name: "Project Stages" })).toBeVisible();
    await expect(page.getByText("Painting", { exact: true })).toBeVisible();
    await shoot(page, "03-settings.png");
  });

  test("create a client", async () => {
    await page.goto("/clients/new");
    await page.selectOption('select[name="type"]', "individual");
    await page.fill('input[name="name"]', "Jane Collector");
    await page.fill('input[name="email"]', "jane@example.com");
    await page.fill('input[name="country"]', "DE");
    await shoot(page, "04-client-new.png");
    await page.click('form[action="/clients"] button[type="submit"]');

    await expect(page).toHaveURL(/\/clients\/[a-f0-9-]+$/);
    clientId = page.url().split("/clients/")[1];
    await expect(page.locator("h1")).toContainText("Jane Collector");
    await shoot(page, "05-client-detail.png");
  });

  test("create an asset with an intake condition state and photo", async () => {
    await page.goto("/assets/new");
    await page.selectOption('select[name="clientId"]', clientId);
    await page.selectOption('select[name="assetTypeId"]', { label: "Painting" });
    await page.fill('input[name="referenceCode"]', "A-0001");
    await page.fill('input[name="title"]', "Sunset over the Bay");
    await page.fill('input[name="artist"]', "J. Doe");
    await page.fill('input[name="dimensions"]', "60x80cm");
    await page.fill('textarea[name="intakeDescription"]', "Minor surface grime, otherwise stable.");
    await page.setInputFiles('input[name="photos"]', TEST_PHOTO);
    await shoot(page, "06-asset-new.png");
    await page.click('form[action="/assets"] button[type="submit"]');

    await expect(page).toHaveURL(/\/assets\/[a-f0-9-]+$/);
    assetId = page.url().split("/assets/")[1];
    await expect(page.locator("h1")).toContainText("Sunset over the Bay");
    await shoot(page, "07-asset-detail.png");
  });

  test("log a treatment on the asset", async () => {
    await page.goto(`/treatments/new?assetId=${assetId}`);
    await page.selectOption('select[name="method"]', { label: "Surface cleaning" });
    await page.fill('input[name="title"]', "Surface cleaning of top layer");
    await page.fill('textarea[name="notes"]', "Removed surface grime with a dry sponge.");
    await shoot(page, "08-treatment-new.png");
    await page.click('form[action="/treatments"] button[type="submit"]');

    await expect(page).toHaveURL(/\/treatments\/[a-f0-9-]+$/);
    treatmentId = page.url().split("/treatments/")[1];
    await expect(page.getByText("Removed surface grime")).toBeVisible();
    await shoot(page, "09-treatment-detail.png");
  });

  test("start a project and move it across the kanban board", async () => {
    await page.goto(`/projects/new?assetId=${assetId}`);
    await page.fill('input[name="title"]', "Conservation of Sunset over the Bay");
    await page.click('form[action="/projects"] button[type="submit"]');

    await expect(page).toHaveURL(/\/projects\/[a-f0-9-]+$/);
    projectId = page.url().split("/projects/")[1];
    await shoot(page, "10-project-detail.png");

    await page.goto("/projects");
    await expect(page.locator('.kanban-column[data-status="inquiry"]')).toBeVisible();
    await shoot(page, "11-projects-kanban.png");

    // Move the project to "working" via the detail page's stage-advance select (exercises the
    // same POST /projects/{id}/stage the kanban board's drag-and-drop uses).
    await page.goto(`/projects/${projectId}`);
    await page.selectOption('form[action$="/stage"] select[name="stage"]', "working");
    await page.click('form[action$="/stage"] button[type="submit"]');
    await expect(page.locator("dd.detail-dd").getByText("Working")).toBeVisible();
    await shoot(page, "12-project-stage-updated.png");
  });

  test("write a report with structured sections and customize its layout", async () => {
    await page.goto(`/reports/new?assetId=${assetId}&projectId=${projectId}`);
    await page.fill('input[name="title"]', "Conservation Report 2026-001");
    await page.click('form[action="/reports"] button[type="submit"]');

    await expect(page).toHaveURL(/\/reports\/[a-f0-9-]+$/);
    reportId = page.url().split("/reports/")[1];
    // The suggested outline pre-fills condition findings/treatment performed from the asset's
    // existing AssetState/Treatment history.
    await expect(page.locator('textarea[name="conditionFindings"]')).not.toBeEmpty();
    await shoot(page, "13-report-detail.png");

    await page.fill('textarea[name="summary"]', "Painting arrived with surface grime; cleaned and stabilized.");
    await page.fill('textarea[name="recommendations"]', "Monitor humidity; reassess in 12 months.");
    await page.click('form[action$="/sections"] button[type="submit"]');
    await expect(page.locator('textarea[name="summary"]')).toHaveValue(/surface grime/);
    await shoot(page, "14-report-sections-saved.png");

    await page.selectOption('form[action$="/layout"] select[name="layoutStyle"]', "gallery");
    await page.click('form[action$="/layout"] button[type="submit"]');
    await shoot(page, "15-report-layout-customized.png");

    await page.click('form[action$="/status"] button[type="submit"]');
    await expect(page.getByText("Final")).toBeVisible();
    await shoot(page, "16-report-final.png");
  });

  test("export the finished report and asset", async () => {
    await page.goto(`/assets/${assetId}`);
    const [htmlDownload] = await Promise.all([
      page.waitForEvent("download"),
      page.click(`a[href="/api/export/asset/${assetId}?format=html"]`),
    ]);
    expect(htmlDownload.suggestedFilename()).toMatch(/\.html$/);

    // page.request (not the top-level `request` fixture) shares this page's session cookie —
    // the export routes are behind RequireUser.
    const pdfResponse = await page.request.get(`/api/export/report/${reportId}?format=pdf`);
    expect(pdfResponse.ok()).toBeTruthy();
    expect(pdfResponse.headers()["content-type"]).toBe("application/pdf");
  });

  test("media grid shows uploaded photos and the lightbox opens", async () => {
    await page.goto("/media");
    await shoot(page, "17-media-grid.png");

    const firstImage = page.locator("a.album-card").first();
    await expect(firstImage).toBeVisible();
    await firstImage.click();
    await expect(page).toHaveURL(/\/media\/view\/[a-f0-9-]+$/);

    await page.click("#lightbox-trigger");
    await expect(page.locator("#lightbox-overlay")).toBeVisible();
    await page.click("#lightbox-rotate");
    await shoot(page, "18-media-lightbox.png");
    await page.click("#lightbox-close");
    await expect(page.locator("#lightbox-overlay")).toBeHidden();
  });

  test("Settings Users table lists accounts", async () => {
    await page.goto("/settings");
    await expect(page.getByText(ADMIN_EMAIL)).toBeVisible();
    await shoot(page, "19-settings-users.png");
  });

  test("mobile viewport renders the single-column layout", async () => {
    await page.setViewportSize({ width: 390, height: 844 });
    await page.goto(`/assets/${assetId}`);
    await shoot(page, "20-mobile-asset-detail.png");
  });
});
