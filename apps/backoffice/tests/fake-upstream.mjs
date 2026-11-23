// A stand-in for backoffice-api in the OIDC journeys. It records what the proxy sent, so a test can check that the proxy
// forwards the bearer token and never identity headers. It answers every /v1 call with empty lists.
import http from "node:http";

let last = null;

http
  .createServer((req, res) => {
    if (req.url === "/__health") return res.end("ok");
    if (req.url === "/__last") {
      res.setHeader("content-type", "application/json");
      return res.end(JSON.stringify(last));
    }
    if (req.url?.startsWith("/v1/")) {
      last = {
        path: req.url,
        authorization: req.headers.authorization ?? null,
        xForgeStaff: req.headers["x-forge-staff"] ?? null,
        xForgeRoles: req.headers["x-forge-roles"] ?? null,
      };
      res.setHeader("content-type", "application/json");
      return res.end(JSON.stringify({ cases: [], breaks: [], reasons: [], break_active: false }));
    }
    res.statusCode = 404;
    res.end();
  })
  .listen(3400, "127.0.0.1");
