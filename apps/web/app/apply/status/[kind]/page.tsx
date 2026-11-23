"use client";

import { useParams } from "next/navigation";
import { useEffect, useState } from "react";
import { ArrowLink, Shell, eyebrowClass, titleClass } from "@/components/apply/Shell";
import { type Application, type Kind, getApplication } from "@/lib/apply";

const STAGES: { step: string; label: string }[] = [
  { step: "prospects_created", label: "Application received" },
  { step: "verifications_started", label: "Identity verification" },
  { step: "kyc_approved", label: "Identity confirmed" },
  { step: "screened", label: "Checks complete" },
  { step: "accounts_created", label: "Account opened" },
];

// Where an application stands. It asks again every few seconds while the application is in progress.
export default function ApplicationStatus() {
  const kind = useParams<{ kind: string }>().kind as Kind;
  const [app, setApp] = useState<Application | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let live = true;
    let timer: ReturnType<typeof setTimeout>;
    const poll = async () => {
      const res = await getApplication(kind);
      if (!live) return;
      if (res.ok) {
        setApp(res.value);
        setError(null);
        if (res.value.state === "in_progress") timer = setTimeout(poll, 4000);
      } else {
        setError(res.status === 404 ? "We could not find an application from you. Start one below." : res.message);
        if (res.status !== 404 && res.status !== 401) timer = setTimeout(poll, 8000);
      }
    };
    void poll();
    return () => {
      live = false;
      clearTimeout(timer);
    };
  }, [kind]);

  const done = new Set(app?.steps ?? []);
  return (
    <Shell>
      <p className={eyebrowClass}>{kind === "business" ? "Business account" : "Personal account"}</p>
      <h1 className={titleClass}>{headline(app)}</h1>

      {error ? (
        <div className="mt-6 space-y-4">
          <p role="alert" className="text-sm text-[#7A2E17]">
            {error}
          </p>
          <ArrowLink href={`/apply/${kind === "business" ? "business" : "personal"}`}>Apply</ArrowLink>
        </div>
      ) : null}

      {app?.next === "verify" && app.verify?.length ? (
        <div className="mt-8 rounded-[2rem] bg-[#1E1E1E] p-8 text-[#FFFFFF]">
          <h2 className="text-lg font-normal">Verify your identity</h2>
          <p className="mt-2 text-sm text-[#C9CBFF]">
            {kind === "business"
              ? "Each link is for one person or for the company. Send each director their own link; it lets whoever opens it complete that verification."
              : "It opens our verification partner in a new tab. Keep this page open; it updates by itself."}
          </p>
          <ul className="mt-4 space-y-2">
            {app.verify.map((v, i) => (
              <li key={v.url}>
                <a
                  href={v.url}
                  target="_blank"
                  rel="noopener noreferrer"
                  className="inline-flex items-center gap-2 rounded-full bg-white px-5 py-2.5 text-sm font-medium text-[#1E1E1E] hover:bg-white"
                >
                  {v.kind === "business" ? "Verify the company" : kind === "business" ? `Verify director ${i}` : "Start verification"}
                  <span aria-hidden="true">↗</span>
                </a>
              </li>
            ))}
          </ul>
        </div>
      ) : null}

      {app && app.state !== "declined" ? (
        <ol className="mt-10 space-y-4 rounded-[2rem] bg-[#F5F5F5] p-8" aria-label="Progress">
          {STAGES.map((s) => (
            <li key={s.step} className="flex items-center gap-3 text-sm">
              <span
                aria-hidden="true"
                className={`h-2.5 w-2.5 rounded-full ${done.has(s.step) ? "bg-[#5A60E8]" : "border border-[#A9A9B2]"}`}
              />
              <span className={done.has(s.step) ? "" : "text-[#5C5C66]"}>
                {s.label}
                <span className="sr-only">{done.has(s.step) ? " (done)" : " (to do)"}</span>
              </span>
            </li>
          ))}
        </ol>
      ) : null}

      {app?.state === "approved" ? (
        <div className="mt-10">
          <ArrowLink href="/apply/welcome" solid>
            Go to your account
          </ArrowLink>
        </div>
      ) : null}
      {app?.state === "declined" ? (
        <p className="mt-6 text-sm text-[#3A3A42]">
          We are not able to open this account. If you think this is wrong, contact us and we will review it.
        </p>
      ) : null}
      {app?.state === "failed" ? (
        <p className="mt-6 text-sm text-[#3A3A42]">Something went wrong on our side. Our team has been told; there is nothing you need to do.</p>
      ) : null}
    </Shell>
  );
}

function headline(app: Application | null): string {
  if (!app) return "Your application";
  switch (app.state) {
    case "approved":
      return "Your account is open";
    case "declined":
      return "We could not open your account";
    case "failed":
      return "Your application is paused";
    default:
      return app.next === "verify" ? "One more step: verify your identity" : "We are checking your application";
  }
}
