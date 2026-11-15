import assert from "node:assert/strict";
import test from "node:test";

import { createDeskApiClient } from "../public/desk/api-client.js";

test("createDeskApiClient sends Authorization Bearer and X-CSRF-Token headers only in memory", async () => {
  const captured = [];
  const fakeFetch = async (url, init) => {
    captured.push({ url, init });
    return {
      ok: true,
      status: 200,
      text: async () => JSON.stringify({ theme: "dark" }),
    };
  };

  const client = createDeskApiClient({
    baseUrl: "http://127.0.0.1:9490",
    bearerToken: "bearer-secret-xyz",
    csrfToken: "csrf-secret-abc",
    fetchImpl: fakeFetch,
  });

  const res = await client.getTheme();
  assert.equal(res.theme, "dark");
  assert.equal(captured.length, 1);
  assert.equal(captured[0].url, "http://127.0.0.1:9490/api/preferences/theme");
  assert.equal(captured[0].init.headers.Authorization, "Bearer bearer-secret-xyz");
  assert.equal(captured[0].init.headers["X-CSRF-Token"], "csrf-secret-abc");
});

test("createDeskApiClient fails closed if session tokens are missing", async () => {
  const client = createDeskApiClient({
    baseUrl: "http://127.0.0.1:9490",
    fetchImpl: async () => {
      throw new Error("should not call fetch without session");
    },
  });

  await assert.rejects(() => client.workspaceTree("."), /session is not yet initialized/i);
});

test("createDeskApiClient surfaces 403 DESK-E403 and 409 DESK-E409 structured errors cleanly", async () => {
  const replayClient = createDeskApiClient({
    baseUrl: "http://127.0.0.1:9490",
    bearerToken: "b",
    csrfToken: "c",
    fetchImpl: async () => ({
      ok: false,
      status: 409,
      text: async () =>
        JSON.stringify({
          ok: false,
          error: { code: "DESK-E409", message: "approval_token already consumed" },
        }),
    }),
  });

  await assert.rejects(
    () =>
      replayClient.groxyApply({
        approval_token: "used-token",
        target_path: "views/workbench.grab.html",
      }),
    (err) => err.status === 409 && err.code === "DESK-E409"
  );
});
