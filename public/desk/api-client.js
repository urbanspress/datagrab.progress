/**
 * Authenticated loopback API client for Groxy Desk v1.0.0.
 * Keeps bearer and CSRF credentials strictly in memory (never localStorage or URL query strings).
 */
export function createDeskApiClient(options = {}) {
  const state = {
    baseUrl: normalizeBaseUrl(options.baseUrl || ""),
    bearerToken: String(options.bearerToken || ""),
    csrfToken: String(options.csrfToken || ""),
  };
  const fetchFn = options.fetchImpl || globalThis.fetch;

  function normalizeBaseUrl(raw) {
    return String(raw || "").replace(/\/+$/, "");
  }

  function setSession(next = {}) {
    if (next.baseUrl !== undefined) {
      state.baseUrl = normalizeBaseUrl(next.baseUrl);
    }
    if (next.bearerToken !== undefined) {
      state.bearerToken = String(next.bearerToken || "");
    }
    if (next.csrfToken !== undefined) {
      state.csrfToken = String(next.csrfToken || "");
    }
  }

  function hasSession() {
    return Boolean(state.bearerToken && state.csrfToken);
  }

  async function request(method, path, body = undefined, requiresAuth = true) {
    if (typeof fetchFn !== "function") {
      throw new Error("Fetch API is unavailable in this environment.");
    }
    const headers = {};
    if (requiresAuth) {
      if (!hasSession()) {
        throw new Error("Authenticated Groxy Desk session is not yet initialized.");
      }
      headers.Authorization = `Bearer ${state.bearerToken}`;
      headers["X-CSRF-Token"] = state.csrfToken;
    }
    let payloadBody;
    if (body !== undefined) {
      headers["Content-Type"] = "application/json";
      payloadBody = JSON.stringify(body);
    }
    const url = `${state.baseUrl}${path}`;
    const res = await fetchFn(url, {
      method,
      headers,
      body: payloadBody,
    });
    const text = await res.text();
    let parsed = {};
    try {
      parsed = text ? JSON.parse(text) : {};
    } catch {
      parsed = { raw: text };
    }
    if (!res.ok) {
      const code = parsed && parsed.error && parsed.error.code ? parsed.error.code : `HTTP-${res.status}`;
      const msg = parsed && parsed.error && parsed.error.message ? parsed.error.message : `Request failed (${res.status})`;
      const err = new Error(`[${code}] ${msg}`);
      err.status = res.status;
      err.code = code;
      err.payload = parsed;
      throw err;
    }
    return parsed;
  }

  return {
    setSession,
    hasSession,
    health: () => request("GET", "/_health", undefined, false),
    ready: () => request("GET", "/_ready", undefined, false),
    getTheme: () => request("GET", "/api/preferences/theme"),
    setTheme: (theme) => request("POST", "/api/preferences/theme", { theme }),
    workspaceTree: (dir = ".") =>
      request("GET", `/api/workspace/tree?dir=${encodeURIComponent(dir)}`),
    workspaceRead: (path) =>
      request("GET", `/api/workspace/read?path=${encodeURIComponent(path)}`),
    workspaceCheck: (grabFile, templateFile = "") =>
      request("POST", "/api/workspace/check", {
        grab_file: grabFile,
        template_file: templateFile,
      }),
    grabmanInitPlan: (payload) =>
      request("POST", "/api/grabman/init/plan", payload),
    grabmanInit: (payload) =>
      request("POST", "/api/grabman/init", payload),
    grabmanDoctor: (targetDir = ".", strict = false) =>
      request("POST", "/api/grabman/doctor", {
        target_dir: targetDir,
        strict: Boolean(strict),
      }),
    grabmanSolveVerify: (targetDir = ".") =>
      request("POST", "/api/grabman/solve-verify", { target_dir: targetDir }),
    listWorkers: () => request("GET", "/api/grisp/workers"),
    startWorker: (payload) =>
      request("POST", "/api/grisp/workers/start", payload),
    reloadWorker: (id) =>
      request("POST", "/api/grisp/workers/reload", { id }),
    stopWorker: (id) =>
      request("POST", "/api/grisp/workers/stop", { id }),
    workerLogs: (id) =>
      request("GET", `/api/grisp/workers/logs?id=${encodeURIComponent(id)}`),
    groxyAsk: (payload) => request("POST", "/api/groxy/ask", payload),
    groxyApply: (payload) =>
      request("POST", "/api/groxy/approvals/apply", payload),
    groxySnapshots: () => request("GET", "/api/groxy/snapshots"),
    groxyRestore: (snapshotId) =>
      request("POST", "/api/groxy/snapshots/restore", {
        snapshot_id: snapshotId,
      }),
    mcpTools: () => request("GET", "/api/mcp/tools"),
    mcpCall: (payload) => request("POST", "/api/mcp/call", payload),
    studioDesignSystem: (studioUrl = "") =>
      request(
        "GET",
        studioUrl
          ? `/api/studio/design-system?studio_url=${encodeURIComponent(studioUrl)}`
          : "/api/studio/design-system"
      ),
    studioSynthesize: (payload) =>
      request("POST", "/api/studio/synthesize", payload),
    studioLayoutApply: (payload) =>
      request("POST", "/api/studio/layout/apply", payload),
    groxyAudit: (payload) => request("POST", "/api/groxy/audit", payload),
    groxyBoost: (payload) => request("POST", "/api/groxy/boost", payload),
    groxyBoostApprove: (payload) =>
      request("POST", "/api/groxy/boost/approve", payload),
    // v0.3.0 Step 5 Fleet, DB Studio, Namecheap DNS/HTTPS & Author Mode endpoints
    fleetNodes: () => request("GET", "/api/fleet/nodes"),
    fleetVerifyNode: (payload) =>
      request("POST", "/api/fleet/nodes/verify", payload),
    fleetValidateDNA: (payload) =>
      request("POST", "/api/fleet/dna/validate", payload),
    fleetDeployPlan: (payload) =>
      request("POST", "/api/fleet/deploy/plan", payload),
    fleetDeployApply: (payload) =>
      request("POST", "/api/fleet/deploy/apply", payload),
    fleetRestoreSnapshot: (payload) =>
      request("POST", "/api/fleet/snapshots/restore", payload),
    dbTunnels: () => request("GET", "/api/db/tunnels"),
    dbTunnelOpen: (payload) =>
      request("POST", "/api/db/tunnels/open", payload),
    dbTunnelClose: (payload) =>
      request("POST", "/api/db/tunnels/close", payload),
    dbSchema: (target) => request("POST", "/api/db/schema", target),
    dbRows: (payload) => request("POST", "/api/db/rows", payload),
    dbQuery: (payload) => request("POST", "/api/db/query", payload),
    dbMutatePlan: (payload) => request("POST", "/api/db/mutate/plan", payload),
    dbMutateApply: (payload) => request("POST", "/api/db/mutate/apply", payload),
    dbRestoreSnapshot: (payload) =>
      request("POST", "/api/db/snapshots/restore", payload),
    dnsVaultStatus: () => request("GET", "/api/dns/vault"),
    dnsVaultSave: (payload) => request("POST", "/api/dns/vault/save", payload),
    dnsPlan: (payload) => request("POST", "/api/dns/plan", payload),
    dnsApply: (payload) => request("POST", "/api/dns/apply", payload),
    httpsPreflight: (payload) =>
      request("POST", "/api/dns/https/preflight", payload),
    httpsApply: (payload) => request("POST", "/api/dns/https/apply", payload),
    dnsRestoreSnapshot: (payload) =>
      request("POST", "/api/dns/snapshots/restore", payload),
    authorInitPlan: (payload) =>
      request("POST", "/api/author/init/plan", payload),
    authorInitApply: (payload) =>
      request("POST", "/api/author/init/apply", payload),
    authorOllamaStatus: (url = "") =>
      request(
        "GET",
        url
          ? `/api/author/ollama/status?url=${encodeURIComponent(url)}`
          : "/api/author/ollama/status"
      ),
    authorOllamaStart: (payload = {}) =>
      request("POST", "/api/author/ollama/start", payload),
  };
}
