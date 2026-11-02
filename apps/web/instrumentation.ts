// Runs when the server starts. A configuration that production must refuse stops the server here, not on the first request.
import { config } from "./lib/config";

export function register() {
  config(process.env);
}
