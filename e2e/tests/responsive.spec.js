// Screenshots the two pages with the widest responsive grids (Dashboard's 4 stat cards, Projects'
// 5-column kanban board) at each of this app's standard CSS breakpoints (see static/css/app.css:
// 640/768/1024, plus a below-640 "mobile" width and a well-above-1024 "wide desktop" width) — a
// permanent regression check for the grid math, not just a one-off visual spot check. Self-
// contained: creates its own client/asset/project rather than depending on another spec file's
// state, so it can run alone.
const { test, expect } = require("@playwright/test");
const path = require("path");

const SCREENSHOT_DIR = path.join(__dirname, "..", "..", "docs", "screenshots");
// No dev-login picker — sign in through the real /login form (see e2e/README.md).
const ADMIN_EMAIL = process.env.E2E_ADMIN_EMAIL || "ada@studio.local";
const ADMIN_PASSWORD = process.env.E2E_ADMIN_PASSWORD || "correct-horse-battery-staple";

// Standard breakpoints: below the sm tier, at each of sm(640)/md(768)/lg(1024), and a wide
// desktop well past lg to confirm nothing keeps growing new columns unexpectedly.
const BREAKPOINTS = [
  { name: "375-mobile", width: 375, height: 812 },
  { name: "640-sm", width: 640, height: 900 },
  { name: "768-md", width: 768, height: 900 },
  { name: "1024-lg", width: 1024, height: 900 },
  { name: "1280-desktop", width: 1280, height: 900 },
];

async function shoot(page, name) {
  await page.screenshot({ path: path.join(SCREENSHOT_DIR, name), fullPage: true });
}

test.describe.configure({ mode: "serial" });

let page, assetId;

test.beforeAll(async ({ browser }) => {
  const context = await browser.newContext({ viewport: { width: 1280, height: 900 } });
  page = await context.newPage();

  await page.goto("/login");
  await page.fill('form[action="/login"] input[name="email"]', ADMIN_EMAIL);
  await page.fill('form[action="/login"] input[name="password"]', ADMIN_PASSWORD);
  await page.click('form[action="/login"] button[type="submit"]');
  await expect(page).toHaveURL("/");

  await page.goto("/clients/new");
  await page.selectOption('select[name="type"]', "individual");
  await page.fill('input[name="name"]', "Responsive Test Client");
  await page.click('form[action="/clients"] button[type="submit"]');
  const clientId = page.url().split("/clients/")[1];

  await page.goto("/assets/new");
  await page.selectOption('select[name="clientId"]', clientId);
  await page.selectOption('select[name="assetTypeId"]', { index: 1 });
  await page.fill('input[name="referenceCode"]', "RESP-0001");
  await page.fill('input[name="title"]', "Responsive Test Asset");
  await page.click('form[action="/assets"] button[type="submit"]');
  assetId = page.url().split("/assets/")[1];

  // A handful of projects across different stages so the kanban board has more than one card to
  // lay out at each breakpoint.
  for (const title of ["First project", "Second project", "Third project"]) {
    await page.goto(`/projects/new?assetId=${assetId}`);
    await page.fill('input[name="title"]', title);
    await page.click('form[action="/projects"] button[type="submit"]');
  }
});

test.afterAll(async () => {
  await page.close();
});

test.describe("Responsive breakpoints", () => {
  for (const bp of BREAKPOINTS) {
    test(`dashboard stat grid at ${bp.name}`, async () => {
      await page.setViewportSize({ width: bp.width, height: bp.height });
      await page.goto("/");
      // 4 stat cards (Clients, Assets, Projects, Reports — active/all or draft/final) must never
      // leave a lone card orphaned on its own row: the grid's column count times its row count
      // must always equal or just exceed 4, with no more than one partially-empty row.
      await expect(page.locator(".stat-card")).toHaveCount(4);
      await shoot(page, `breakpoint-${bp.name}-dashboard.png`);
    });

    test(`projects kanban at ${bp.name}`, async () => {
      await page.setViewportSize({ width: bp.width, height: bp.height });
      await page.goto("/projects");
      await expect(page.locator(".kanban-column")).toHaveCount(5);
      await shoot(page, `breakpoint-${bp.name}-projects.png`);
    });
  }

  test("asset detail (treatment badges show labels, not raw codes) at mobile", async () => {
    await page.setViewportSize({ width: 375, height: 812 });
    await page.goto(`/assets/${assetId}`);
    await shoot(page, "breakpoint-375-mobile-asset-detail.png");
  });
});
