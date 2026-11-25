"use client";

import { usePathname } from "next/navigation";
import type { ReactNode } from "react";

/** Replays a short entrance each time the page changes. */
export default function PageMotion({ children }: { children: ReactNode }) {
  const pathname = usePathname();
  return (
    <div key={pathname} className="motion-page">
      {children}
    </div>
  );
}
