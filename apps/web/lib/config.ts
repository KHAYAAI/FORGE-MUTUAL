// Runtime configuration, read from the environment on each call, so one build serves every environment.
//
// FORGE_AUTH_MODE is "oidc" (the default) or "dev". Dev sign-in trusts a name and roles typed into a form; it is refused in
// production, and the server refuses to start with it (see instrumentation.ts). OIDC needs an issuer, an audience and a
// client id; in production the public address must be https.

export type AuthMode = "oidc" | "dev";

export type Config = {
  mode: AuthMode;
  production: boolean;
  publicUrl: string;
  issuer: string;
  audience: string;
  clientId: string;
  clientSecret: string;
};

export function config(env: Record<string, string | undefined> = process.env): Config {
  const production = env.FORGE_ENV === "production";
  const raw = env.FORGE_AUTH_MODE ?? "oidc";
  if (raw !== "oidc" && raw !== "dev") {
    throw new Error(`FORGE_AUTH_MODE ${raw} is not a known mode`);
  }
  if (raw === "dev" && production) {
    throw new Error("dev sign-in is refused in production");
  }
  const publicUrl = env.FORGE_PUBLIC_URL ?? "";
  if (raw === "oidc") {
    for (const name of ["OIDC_ISSUER", "OIDC_AUDIENCE", "OIDC_CLIENT_ID", "FORGE_PUBLIC_URL"]) {
      if (!env[name]) throw new Error(`${name} is required when FORGE_AUTH_MODE is oidc`);
    }
    if (production && !publicUrl.startsWith("https://")) {
      throw new Error("FORGE_PUBLIC_URL must be https in production");
    }
  }
  return {
    mode: raw,
    production,
    publicUrl,
    issuer: env.OIDC_ISSUER ?? "",
    audience: env.OIDC_AUDIENCE ?? "",
    clientId: env.OIDC_CLIENT_ID ?? "",
    clientSecret: env.OIDC_CLIENT_SECRET ?? "",
  };
}
