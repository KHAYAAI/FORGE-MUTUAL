import { config } from "@/lib/config";

// The sign-in page asks which way to sign in. It reveals the mode only, never the configuration.
export async function GET() {
  return Response.json({ mode: config().mode });
}
