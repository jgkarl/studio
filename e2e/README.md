# End-to-end tests

Playwright suite driven against a live server — never against mocks. The app needs libvips
(govips/cgo), so the server under test always runs in the Debian/libvips Docker image, not
directly on the host.

## Run

```sh
# 1. Build and start the app with a throwaway database and a bootstrap admin:
make release   # or: docker build -t studio-go .
docker run -d --name studio-e2e -p 4070:3000 \
  -e AUTH_SECRET=some-random-string \
  -e ALLOW_DEV_LOGIN=true \
  -e BOOTSTRAP_ADMIN_NAME="Ada Admin" \
  -e BOOTSTRAP_ADMIN_EMAIL="ada@studio.local" \
  -v "$(mktemp -d):/data" \
  studio-go

# 2. Run the suite against it:
cd e2e
npm install
BASE_URL=http://localhost:4070 npm test

# 3. Tear down:
docker rm -f studio-e2e
```

Uses the system-installed Chromium (`/snap/bin/chromium` by default — override with
`CHROMIUM_PATH`) instead of a Playwright-managed browser download, so no `playwright install`
step is needed.

`tests/smoke.spec.js` is one continuous session through the golden path — login, settings
(classifier chip-groups), client, asset (with an intake photo), treatment, project (kanban stage
moves), report (structured sections + customize layout), export, media grid + lightbox, user
management, and a mobile viewport check — screenshotting each step into `docs/screenshots/`.
Tests run serially in a single shared browser context (real login cookie carried through, same as
one person's browser tab) and each depends on state the previous one created, so a fresh database
is required for a clean run — a stale `A-0001` reference code from a prior run will fail asset
creation with a UNIQUE constraint error.
