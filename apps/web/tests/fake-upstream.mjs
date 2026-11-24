// A stand-in for workspace-api in the OIDC journeys. It records what the proxy sent, so a test can check that the proxy
// forwards the bearer token and never identity headers. It answers every /v1 call with an empty account list.
import http from "node:http";

let last = null;
const byPath = new Map(); // the last request for each path, so a test can ask about one call whatever else the page also fetched

http
  .createServer((req, res) => {
    if (req.url === "/__health") return res.end("ok");
    if (req.url?.startsWith("/__last")) {
      res.setHeader("content-type", "application/json");
      const prefix = new URL(req.url, "http://x").searchParams.get("prefix");
      if (!prefix) return res.end(JSON.stringify(last));
      const hit = [...byPath.entries()].filter(([p]) => p.startsWith(prefix)).map(([, v]) => v).at(-1) ?? null;
      return res.end(JSON.stringify(hit));
    }
    if (req.url?.startsWith("/v1/")) {
      last = {
        path: req.url,
        authorization: req.headers.authorization ?? null,
        xForgeParty: req.headers["x-forge-party"] ?? null,
        xForgeActor: req.headers["x-forge-actor"] ?? null,
      };
      byPath.set(req.url, last);
      res.setHeader("content-type", "application/json");
      return res.end(JSON.stringify({ accounts: [] }));
    }
    res.statusCode = 404;
    res.end();
  })
  .listen(3400, "127.0.0.1");
