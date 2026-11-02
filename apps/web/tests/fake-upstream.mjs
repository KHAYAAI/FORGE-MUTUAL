// A stand-in for workspace-api in the OIDC journeys. It records what the proxy sent, so a test can check that the proxy
// forwards the bearer token and never identity headers. It answers every /v1 call with an empty account list.
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
        xForgeParty: req.headers["x-forge-party"] ?? null,
        xForgeActor: req.headers["x-forge-actor"] ?? null,
      };
      res.setHeader("content-type", "application/json");
      return res.end(JSON.stringify({ accounts: [] }));
    }
    res.statusCode = 404;
    res.end();
  })
  .listen(3400, "127.0.0.1");
