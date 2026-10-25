import { expect, test } from "@playwright/test";

const visualStates = ["active", "crashed", "idle", "mixed", "partial-failure", "saturated", "stale", "truncated"] as const;

for (const fixture of visualStates) {
  test(`${fixture} state matches its reference viewport`, async ({ page }) => {
    await page.setViewportSize({ width: 1280, height: 1080 });
    await page.goto(`/?fixture=${fixture}`);
    await expect(page.getByRole("heading", { name: /Agent Substrate/ })).toBeVisible();
    await expect(page.locator("substrate-dashboard")).toHaveScreenshot(`${fixture}-1280x1080.png`, {
      animations: "disabled",
      caret: "hide",
      scale: "css",
    });
    const dimensions = await page.evaluate(() => ({
      height: document.documentElement.clientHeight,
      scrollHeight: document.documentElement.scrollHeight,
    }));
    expect(dimensions.scrollHeight).toBe(dimensions.height);
  });
}

test("actor selection enables only backend-supported operations", async ({ page }) => {
  await page.goto("/?fixture=mixed");
  await page.getByRole("checkbox", { name: "Select actor suspended" }).check();
  await expect(page.getByRole("button", { name: "Resume 1" })).toBeEnabled();
  await expect(page.getByRole("button", { name: "Suspend 1" })).toBeEnabled();
  await page.getByRole("button", { name: "Resume 1" }).click();
  await expect(page.getByRole("status")).toContainText("Fixture mode");

  await page.goto("/?fixture=idle");
  await expect(page.getByText("Observe-only mode · lifecycle operations are disabled by backend policy.")).toBeVisible();
  await page.getByRole("checkbox", { name: "Select actor agent-a" }).check();
  await expect(page.getByRole("button", { name: "Resume" })).toBeDisabled();
  await expect(page.getByRole("button", { name: "Suspend" })).toBeDisabled();
});

test("partial operation results are shown without being reported as atomic success", async ({ page }) => {
  const fixtureResponse = await page.request.get("/snapshots/mixed.json");
  const fixture = await fixtureResponse.json();
  let operation: Record<string, unknown> | undefined;
  let requestedActors: string[] = [];
  await page.route("**/api/v1/meta", route => route.fulfill({
    status: 200,
    contentType: "application/json",
    body: JSON.stringify({ apiVersion: "v1", componentVersion: "test", supportedSubstrateVersion: "test", capabilities: ["OBSERVE", "RESUME", "SUSPEND"], scope: fixture.scope }),
  }));
  await page.route("**/api/v1/snapshot", route => route.fulfill({
    status: 200,
    contentType: "application/json",
    body: JSON.stringify({ ...fixture, operations: operation ? [operation] : [] }),
  }));
  await page.route("**/api/v1/operations/resume", async route => {
    requestedActors = (await route.request().postDataJSON()).actorNames;
    operation = {
      id: "op-partial", type: "RESUME", requestedAt: fixture.observedAt, completedAt: fixture.observedAt,
      results: [
        { actorName: "suspended", outcome: "SUCCEEDED", message: "" },
        { actorName: "running", outcome: "FAILED", message: "actor state changed concurrently" },
      ],
    };
    await route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ operation }) });
  });
  await page.goto("/");
  await page.getByRole("checkbox", { name: "Select actor suspended" }).check();
  await page.getByRole("checkbox", { name: "Select actor running" }).check();
  await page.getByRole("button", { name: "Resume 2" }).click();
  await expect(page.getByRole("status")).toContainText("1 of 2 requests failed");
  await expect(page.getByRole("region", { name: "Dashboard operations" })).toContainText("actor state changed concurrently");
  expect(requestedActors).toEqual(["suspended", "running"]);
});

test("unsupported API versions fail closed", async ({ page }) => {
  await page.route("**/api/v1/meta", route => route.fulfill({
    status: 200,
    contentType: "application/json",
    body: JSON.stringify({
      apiVersion: "v2",
      componentVersion: "2.0.0",
      supportedSubstrateVersion: "future",
      capabilities: ["OBSERVE"],
      scope: { atespace: "demo", workerPool: "agents" },
    }),
  }));
  await page.goto("/");
  await expect(page.getByRole("alert")).toContainText("Frontend supports v1; backend reported v2");
  await expect(page.getByRole("region", { name: "Fleet summary" })).toHaveCount(0);
});

test("narrow viewport reflows without horizontal page overflow", async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto("/?fixture=mixed");
  const dimensions = await page.evaluate(() => ({
    clientWidth: document.documentElement.clientWidth,
    scrollWidth: document.documentElement.scrollWidth,
    scrollHeight: document.documentElement.scrollHeight,
  }));
  expect(dimensions.scrollWidth).toBe(dimensions.clientWidth);
  expect(dimensions.scrollHeight).toBeGreaterThan(844);
  await expect(page.getByRole("region", { name: "Actor fleet" })).toBeVisible();
});

test("a failed poll retains the last complete snapshot and marks it stale", async ({ page }) => {
  const fixtureResponse = await page.request.get("/snapshots/mixed.json");
  const snapshot = await fixtureResponse.json();
  let observations = 0;
  await page.route("**/api/v1/meta", route => route.fulfill({
    status: 200,
    contentType: "application/json",
    body: JSON.stringify({
      apiVersion: "v1",
      componentVersion: "0.1.0",
      supportedSubstrateVersion: "test",
      capabilities: ["OBSERVE"],
      scope: { atespace: "demo", workerPool: "agents" },
    }),
  }));
  await page.route("**/api/v1/snapshot", route => {
    observations += 1;
    return observations === 1
      ? route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify(snapshot) })
      : route.fulfill({ status: 503, contentType: "application/json", body: JSON.stringify({ code: "unavailable", message: "observation failed" }) });
  });
  await page.goto("/");
  await expect(page.getByRole("region", { name: "Actor fleet" })).toBeVisible();
  await expect(page.getByRole("region", { name: "Control API" })).toContainText("Stale", { timeout: 5_000 });
  await expect(page.getByRole("region", { name: "Actor fleet" })).toContainText("suspended");
});

test("fixture gallery renders every card independently", async ({ page }) => {
  await page.goto("/gallery.html?fixture=partial-failure");
  const examples = [
    "actor-status-card", "control-api-card", "transition-count-card", "worker-occupancy-card",
    "operational-efficiency-card", "actor-lifecycle-card", "worker-map-card", "actor-fleet-card",
    "actor-timeline-card", "operation-stream-card",
  ];
  for (const tag of examples) await expect(page.locator(`fixture-gallery ${tag}`)).toHaveCount(1);
  await expect(page.getByText("no worker capacity available")).toBeVisible();
});

test("independent cards remain container-safe at 320px", async ({ page }) => {
  await page.setViewportSize({ width: 320, height: 844 });
  await page.goto("/gallery.html?fixture=mixed");
  const overflows = await page.locator("fixture-gallery article > :not(div)").evaluateAll(elements => elements
    .filter(element => element.scrollWidth > element.clientWidth)
    .map(element => element.tagName.toLowerCase()));
  expect(overflows).toEqual([]);
  await expect(page.locator("fixture-gallery actor-fleet-card")).toBeVisible();
});
