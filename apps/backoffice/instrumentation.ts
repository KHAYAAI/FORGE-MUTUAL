// Runs when the server starts. A configuration that production must refuse stops the server here, not on the first request.
import { config } from "./lib/config";

export async function register() {
  config(process.env);
  // The check reads files with Node APIs, so it runs only in the Node runtime, which is where the API calls are made.
  if (process.env.NEXT_RUNTIME !== "nodejs") return;
  const { upstreamPolicy } = await import("./lib/upstream");
  upstreamPolicy(process.env.BACKOFFICE_API_URL ?? "http://127.0.0.1:8500", process.env);
}
