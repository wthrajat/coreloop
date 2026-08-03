"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";

import { navigationItems } from "@/lib/navigation";

function isCurrentPath(pathname: string, href: string): boolean {
  return pathname === href || pathname.startsWith(`${href}/`);
}

export function AppNavigation({ owner = false }: { owner?: boolean }) {
  const pathname = usePathname();

  const items = owner
    ? [
        ...navigationItems,
        { href: "/operations", label: "Operations", mobileLabel: "Ops" },
      ]
    : navigationItems;

  return (
    <nav aria-label="Primary navigation" className="app-navigation">
      {items.map((item) => {
        const isCurrent = isCurrentPath(pathname, item.href);

        return (
          <Link
            aria-current={isCurrent ? "page" : undefined}
            className="navigation-link"
            data-current={isCurrent ? "true" : "false"}
            href={item.href}
            key={item.href}
          >
            <span className="navigation-indicator" aria-hidden="true" />
            <span className="navigation-label">{item.label}</span>
            <span className="navigation-label-mobile">{item.mobileLabel}</span>
          </Link>
        );
      })}
    </nav>
  );
}
