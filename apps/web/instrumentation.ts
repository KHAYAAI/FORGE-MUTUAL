// Runs when the server starts. A configuration that production must refuse stops the server here, not on the first request.
import { config } from "./lib/config";
import { upstreamPolicy } from "./lib/upstream";

export function register() {
  config(process.env);
  upstreamPolicy(process.env.WORKSPACE_API_URL ?? "http://127.0.0.1:8300", process.env);
}
