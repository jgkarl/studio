// End-to-end walk through the app's golden path, module by module, against a live server
// (Docker container running the real Debian/libvips build — see e2e/README.md). Serial: each
// step builds on state the previous one created (client -> asset -> workflow -> order -> report),
// same as a real conservator's session would. Screenshots land in docs/screenshots/, numbered in
// the order a first-time user would actually see these pages.
const { test, expect } = require("@playwright/test");
const path = require("path");

const SCREENSHOT_DIR = path.join(__dirname, "..", "..", "docs", "screenshots");
// The dev-login picker shows each user's name + role (not email — see internal/auth/views.templ's
// devLoginList), while Settings -> Users shows the email — set both via BOOTSTRAP_ADMIN_NAME/
// BOOTSTRAP_ADMIN_EMAIL when starting the server for this suite.
const ADMIN_NAME = process.env.E2E_ADMIN_NAME || "Ada Admin";
const ADMIN_EMAIL = process.env.E2E_ADMIN_EMAIL || "ada@studio.local";

async function shoot(page, name) {
  await page.screenshot({ path: path.join(SCREENSHOT_DIR, name), fullPage: true });
}

test.describe.configure({ mode: "serial" });

let clientId, assetId, projectId, orderId, reportId;

// A single shared page/context across every test in this file (rather than each test's own
// `page` fixture, which starts a fresh, unauthenticated context) — the whole point of this suite
// is one continuous session through the app, logging in once and carrying that session's cookie
// through client -> asset -> workflow -> order -> report, the way a real user's browser tab would.
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

  test("Settings shows the seeded classifiers", async () => {
    await page.goto("/settings");
    await expect(page.getByText(/options across/i)).toBeVisible();
    await shoot(page, "03-settings.png");

    await page.goto("/settings/classifiers/asset_type");
    await expect(page.getByRole("cell", { name: "Painting", exact: true })).toBeVisible();
    await shoot(page, "04-settings-classifiers.png");
  });

  test("create a client", async () => {
    await page.goto("/clients/new");
    await page.selectOption('select[name="type"]', "individual");
    await page.fill('input[name="name"]', "Jane Collector");
    await page.fill('input[name="email"]', "jane@example.com");
    await page.fill('input[name="country"]', "DE");
    await shoot(page, "05-client-new.png");
    await page.click('form[action="/clients"] button[type="submit"]');

    await expect(page).toHaveURL(/\/clients\/[a-f0-9-]+$/);
    clientId = page.url().split("/clients/")[1];
    await expect(page.locator("h1")).toContainText("Jane Collector");
    await shoot(page, "06-client-detail.png");
  });

  test("create an asset with an intake condition state", async () => {
    await page.goto("/assets/new");
    await page.selectOption('select[name="clientId"]', clientId);
    await page.selectOption('select[name="assetTypeId"]', { label: "Painting" });
    await page.fill('input[name="referenceCode"]', "A-0001");
    await page.fill('input[name="title"]', "Sunset over the Bay");
    await page.fill('input[name="artist"]', "J. Doe");
    await page.fill('input[name="dimensions"]', "60x80cm");
    await page.fill('textarea[name="intakeDescription"]', "Minor surface grime, otherwise stable.");
    await shoot(page, "07-asset-new.png");
    await page.click('form[action="/assets"] button[type="submit"]');

    await expect(page).toHaveURL(/\/assets\/[a-f0-9-]+$/);
    assetId = page.url().split("/assets/")[1];
    await expect(page.locator("h1")).toContainText("Sunset over the Bay");
    await shoot(page, "08-asset-detail.png");
  });

  test("start a workflow and log an activity", async () => {
    await page.goto(`/workflows/new?assetId=${assetId}`);
    await page.fill('input[name="title"]', "Conservation of Sunset over the Bay");
    await page.click('form[action="/workflows"] button[type="submit"]');

    await expect(page).toHaveURL(/\/workflows\/[a-f0-9-]+$/);
    projectId = page.url().split("/workflows/")[1];
    await shoot(page, "09-workflow-detail.png");

    await page.selectOption('form[action$="/activities"] select[name="activityTypeId"]', {
      label: "Surface Cleaning",
    });
    await page.fill('form[action$="/activities"] textarea[name="description"]', "Removed surface grime with a dry sponge.");
    await page.fill('form[action$="/activities"] input[name="durationMinutes"]', "45");
    await page.click('form[action$="/activities"] button[type="submit"]');

    await expect(page.getByText("Removed surface grime")).toBeVisible();
    await shoot(page, "10-workflow-activity-logged.png");
  });

  test("quote, accept into an order, and invoice", async () => {
    await page.goto(`/clients/quotes/new?clientId=${clientId}`);
    await page.fill('input[name="item_description_0"]', "Cleaning and consolidation");
    await page.fill('input[name="item_hours_0"]', "4");
    await page.fill('input[name="item_rate_0"]', "80");
    await shoot(page, "11-quote-new.png");
    await page.click('form[action="/clients/quotes"] button[type="submit"]');
    await expect(page).toHaveURL(/\/clients\/quotes\/[a-f0-9-]+$/);
    await shoot(page, "12-quote-detail.png");

    await page.click('form[action$="/accept"] button[type="submit"]');
    await expect(page).toHaveURL(/\/clients\/orders\/[a-f0-9-]+$/);
    orderId = page.url().split("/clients/orders/")[1];
    await shoot(page, "13-order-detail.png");

    // The "Create invoice" form lives inside a collapsed <details> — open it first.
    await page.click("details.form-section summary");
    await page.fill('form[action$="/invoices"] input[name="item_description_0"]', "Cleaning and consolidation");
    await page.fill('form[action$="/invoices"] input[name="item_hours_0"]', "4");
    await page.fill('form[action$="/invoices"] input[name="item_rate_0"]', "80");
    await page.click('form[action$="/invoices"] button[type="submit"]');
    await expect(page.getByText(/EUR/)).toBeVisible();
    await shoot(page, "14-order-invoiced.png");

    await page.goto("/clients/orders");
    await shoot(page, "15-orders-kanban.png");
  });

  test("write and finalize a conservation report", async () => {
    await page.goto(`/reporter/new?assetId=${assetId}&projectId=${projectId}`);
    await page.fill('input[name="title"]', "Conservation Report 2026-001");
    await page.click('form[action="/reporter"] button[type="submit"]');

    await expect(page).toHaveURL(/\/reporter\/[a-f0-9-]+$/);
    reportId = page.url().split("/reporter/")[1];

    // TipTap loads live via esm.sh's import map — give it a moment to mount before screenshotting.
    await page.waitForSelector(".ProseMirror", { timeout: 10_000 });
    await shoot(page, "16-report-editor.png");

    await page.click(".report-editor-content .ProseMirror");
    await page.keyboard.type(" Reviewed and confirmed by the lead conservator.");
    await page.click(".report-editor-save");
    await expect(page.locator(".report-editor-save")).toHaveText(/saved/i, { timeout: 5_000 });

    await page.click('form[action$="/status"] button[type="submit"]');
    await expect(page.getByText("Final")).toBeVisible();
    await shoot(page, "17-report-final.png");
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

  test("album shows uploaded media", async () => {
    await page.goto("/album");
    await shoot(page, "18-album.png");
  });

  test("Settings -> Users lists accounts", async () => {
    await page.goto("/settings/users");
    await expect(page.getByText(ADMIN_EMAIL)).toBeVisible();
    await shoot(page, "19-settings-users.png");
  });

  test("mobile viewport renders the single-column layout", async () => {
    await page.setViewportSize({ width: 390, height: 844 });
    await page.goto(`/assets/${assetId}`);
    await shoot(page, "20-mobile-asset-detail.png");
  });
});
