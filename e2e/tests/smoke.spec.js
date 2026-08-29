// End-to-end walk through the app's golden path, module by module, against a live server
// (Docker container running the real Debian/libvips build — see e2e/README.md). Serial: each
// step builds on state the previous one created (client -> asset -> project -> assessment ->
// treatment -> report), same as a real conservator's session would — Project is now the
// mandatory parent for Assessments/Treatments/Reports, so an Asset's first Project always comes
// right after creating it. Screenshots land in docs/screenshots/, numbered in the order a
// first-time user would actually see these pages.
const { test, expect } = require("@playwright/test");
const path = require("path");

const SCREENSHOT_DIR = path.join(__dirname, "..", "..", "docs", "screenshots");
const TEST_PHOTO = path.join(__dirname, "..", "fixtures", "test-photo.png");
// No dev-login picker — sign in through the real /login form, same as any account. Set all three
// via BOOTSTRAP_ADMIN_NAME/EMAIL/PASSWORD when starting the server for this suite (see
// e2e/README.md).
const ADMIN_EMAIL = process.env.E2E_ADMIN_EMAIL || "ada@studio.local";
const ADMIN_PASSWORD = process.env.E2E_ADMIN_PASSWORD || "correct-horse-battery-staple";

async function shoot(page, name) {
  await page.screenshot({ path: path.join(SCREENSHOT_DIR, name), fullPage: true });
}

// Fills a ClassifierAutocomplete field (internal/web/ui.templ) by its `name` - these replaced
// plain <select>s app-wide (see "UI refactor: full-width lists, classifier autocomplete, link/
// select polish"), so there's no <select> to target any more. Typing a title already in the
// <datalist> and blurring (static/js/classifier-autocomplete.js) resolves it into the hidden
// field the form actually submits.
async function fillClassifierAutocomplete(page, name, title) {
  await page.fill(`input[list="${name}-options"]`, title);
  await page.locator(`input[list="${name}-options"]`).press("Tab");
}

test.describe.configure({ mode: "serial" });

let clientId, assetId, projectId, treatmentId, reportId;

// A single shared page/context across every test in this file (rather than each test's own
// `page` fixture, which starts a fresh, unauthenticated context) — the whole point of this suite
// is one continuous session through the app, logging in once and carrying that session's cookie
// through client -> asset -> project -> treatment -> report.
let page;

test.beforeAll(async ({ browser }) => {
  const context = await browser.newContext();
  page = await context.newPage();
  // Several forms in this golden path (e.g. "Finish project") carry data-confirm
  // (static/js/confirm.js), which calls the native confirm() before submitting - Playwright
  // auto-dismisses unhandled dialogs, which would silently no-op every one of those submits.
  page.on("dialog", (dialog) => dialog.accept());
});

test.afterAll(async () => {
  await page.close();
});

test.describe("Studio golden path", () => {
  test("log in as the bootstrapped admin", async () => {
    await page.goto("/login");
    await expect(page.locator("h1")).toContainText("Studio");
    await shoot(page, "01-login.png");

    await page.fill('form[action="/login"] input[name="email"]', ADMIN_EMAIL);
    await page.fill('form[action="/login"] input[name="password"]', ADMIN_PASSWORD);
    await page.click('form[action="/login"] button[type="submit"]');
    await expect(page).toHaveURL("/");
    await shoot(page, "02-dashboard.png");
  });

  test("Settings Classifiers tab lists the seeded classifier groups", async () => {
    await page.goto("/settings");
    await expect(page.getByRole("heading", { name: "Asset Types" })).toBeVisible();
    await expect(page.getByRole("heading", { name: "Treatment Methods" })).toBeVisible();
    await expect(page.getByRole("heading", { name: "Project Stages" })).toBeVisible();
    // Scoped to the pill, not getByText(exact) - the pill also contains icon-ligature text
    // ("edit"/"close" from the Material Symbols glyphs) so its full text isn't just "Painting".
    await expect(page.locator(".tag-pill").filter({ hasText: "Painting" }).first()).toBeVisible();
    await shoot(page, "03-settings.png");
  });

  test("create a client", async () => {
    await page.goto("/clients/new");
    await fillClassifierAutocomplete(page, "type", "Individual");
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

  test("create an asset, which leads straight into starting its first project", async () => {
    await page.goto("/assets/new");
    await page.selectOption('select[name="clientId"]', clientId);
    await fillClassifierAutocomplete(page, "assetTypeId", "Painting");
    await page.fill('input[name="referenceCode"]', "A-0001");
    await page.fill('input[name="title"]', "Sunset over the Bay");
    await page.fill('input[name="artist"]', "J. Doe");
    await page.fill('input[name="dimensions"]', "60x80cm");
    await shoot(page, "06-asset-new.png");
    await page.click('form[action="/assets"] button[type="submit"]');

    // Registering an Asset redirects into New Project (Project is now the mandatory scope for
    // Assessments/Treatments/Reports) with the asset preselected.
    await expect(page).toHaveURL(/\/projects\/new\?assetId=/);
    assetId = new URL(page.url()).searchParams.get("assetId");

    await page.fill('input[name="title"]', "Conservation of Sunset over the Bay");
    // The New Project form's optional "Initial assessment" sub-block records the intake
    // condition as this project's first Assessment, right after the project itself is created.
    await fillClassifierAutocomplete(page, "assessmentCondition", "Excellent");
    await page.fill('textarea[name="assessmentDescription"]', "Minor surface grime, otherwise stable.");
    await page.setInputFiles('input[name="photos"]', TEST_PHOTO);
    await shoot(page, "07-project-new-with-assessment.png");
    await page.click('form[action="/projects"] button[type="submit"]');

    await expect(page).toHaveURL(/\/projects\/[a-f0-9-]+$/);
    projectId = page.url().split("/projects/")[1];
    await expect(page.locator("h1")).toContainText("Conservation of Sunset over the Bay");
    await shoot(page, "08-project-detail-with-assessment.png");
  });

  test("log a treatment on the project", async () => {
    await page.goto(`/treatments/new?projectId=${projectId}`);
    await fillClassifierAutocomplete(page, "method", "Surface cleaning");
    await page.fill('input[name="title"]', "Surface cleaning of top layer");
    await page.fill('textarea[name="notes"]', "Removed surface grime with a dry sponge.");
    await shoot(page, "09-treatment-new.png");
    await page.click('form[action="/treatments"] button[type="submit"]');

    await expect(page).toHaveURL(/\/treatments\/[a-f0-9-]+$/);
    treatmentId = page.url().split("/treatments/")[1];
    await expect(page.getByText("Removed surface grime")).toBeVisible();
    await shoot(page, "10-treatment-detail.png");
  });

  test("move the project across the kanban board", async () => {
    await page.goto("/projects");
    await expect(page.locator('.kanban-column[data-status="inquiry"]')).toBeVisible();
    await shoot(page, "11-projects-kanban.png");

    // Move the project to "working" via the detail page's stage-advance select (exercises the
    // same POST /projects/{id}/stage the kanban board's drag-and-drop uses). "completed" is
    // deliberately not an option here — finishing a project goes through its own
    // POST /projects/{id}/finish action instead (see the next test).
    await page.goto(`/projects/${projectId}`);
    await page.selectOption('form[action$="/stage"] select[name="stage"]', "working");
    await page.click('form[action$="/stage"] button[type="submit"]');
    await expect(page.locator("dd.detail-dd").getByText("Working")).toBeVisible();
    await shoot(page, "12-project-stage-updated.png");
  });

  test("write a report with structured sections and customize its layout", async () => {
    await page.goto(`/reports/new?projectId=${projectId}`);
    await page.fill('input[name="title"]', "Conservation Report 2026-001");
    await page.click('form[action="/reports"] button[type="submit"]');

    await expect(page).toHaveURL(/\/reports\/[a-f0-9-]+$/);
    reportId = page.url().split("/reports/")[1];
    // The structured-section editor (description/summary/recommendations - the old freeform
    // conditionFindings/treatmentPerformed fields were dropped in the reporter overhaul).
    await expect(page.locator('textarea[name="summary"]')).toBeVisible();
    // The Assessments/Treatments tables and the timestamp-ordered image gallery are rendered
    // live from the project, not copied into the report.
    await expect(page.getByRole("heading", { name: "Assessments" })).toBeVisible();
    await expect(page.getByRole("heading", { name: "Treatments" })).toBeVisible();
    await expect(page.getByRole("heading", { name: "Image gallery" })).toBeVisible();
    await expect(page.getByRole("heading", { name: "Client / Asset / Project info" })).toBeVisible();
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

  test("finishing a project that already has a report does not draft a second one", async () => {
    await page.goto(`/projects/${projectId}`);
    await page.click('form[action$="/finish"] button[type="submit"]');
    // A Report already exists (created above), so finishing just marks the project completed and
    // returns to its own detail page rather than drafting/redirecting to a new report.
    await expect(page).toHaveURL(new RegExp(`/projects/${projectId}$`));
    await expect(page.locator("dd.detail-dd").getByText("Completed")).toBeVisible();
    await shoot(page, "16b-project-finished.png");
  });

  test("media grid shows uploaded photos and the deep-zoom edit page works", async () => {
    await page.goto("/media");
    await shoot(page, "17-media-grid.png");

    const firstImage = page.locator("a.album-card").first();
    await expect(firstImage).toBeVisible();
    await firstImage.click();
    await expect(page).toHaveURL(/\/media\/view\/[a-f0-9-]+$/);

    // media/view/<id> embeds a deep-zoom viewer directly on the page — no click-to-open modal
    // needed (see internal/media/views.templ's mediaViewBody / static/js/media-viewer.js).
    await expect(page.locator("#media-view-stage")).toBeVisible();

    // A true original is view-only - annotating it means starting an annotated version first (see
    // annotatedVersionsCard/CreateAnnotatedVersion), which now lands straight on its own edit page
    // (/media/edit/{id}) to actually draw on, rather than opening a modal.
    await page.click('form[action*="/annotated-versions"] button[type="submit"]');
    await expect(page).toHaveURL(/\/media\/edit\/[a-f0-9-]+$/);

    // The stage defaults to plain pan/zoom now - drawing is only armed via "+ New annotation",
    // which reveals the draft panel (see static/js/media-editor.js).
    const draft = page.locator("#annotation-draft");
    await expect(draft).toBeHidden();
    await page.click("#annotation-new");
    await expect(draft).toBeVisible();
    // Existing annotations (none yet on a fresh version) are listed in the table below the image.
    await expect(page.locator("#annotations-table")).toBeVisible();
    await shoot(page, "18-media-lightbox.png");

    // "Save & finish" persists the whole-image note + bakes the version, then returns to view.
    await page.click("#media-edit-save");
    await expect(page).toHaveURL(/\/media\/view\/[a-f0-9-]+$/);
    await expect(page.locator("a.btn:has-text('Edit')")).toBeVisible();
  });

  test("Settings Users table lists accounts", async () => {
    // Users has its own tab (/settings/users) alongside Classifiers (/settings) and Features
    // (/settings/features) — see internal/settings/views.templ's tabNav.
    await page.goto("/settings/users");
    await expect(page.getByText(ADMIN_EMAIL)).toBeVisible();
    await shoot(page, "19-settings-users.png");
  });

  test("Settings Features tab configures dashboard limits and reportable fields", async () => {
    await page.goto("/settings/features");
    await expect(page.getByRole("heading", { name: "Dashboard limits" })).toBeVisible();
    await expect(page.getByRole("heading", { name: "Reportable fields" })).toBeVisible();
    await shoot(page, "19b-settings-features.png");
  });

  test("mobile viewport renders the single-column layout", async () => {
    await page.setViewportSize({ width: 390, height: 844 });
    await page.goto(`/assets/${assetId}`);
    await shoot(page, "20-mobile-asset-detail.png");
  });
});
