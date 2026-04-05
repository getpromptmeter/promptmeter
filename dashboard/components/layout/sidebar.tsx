"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { LayoutDashboard, Settings } from "lucide-react";
import { cn } from "@/lib/utils";

const navItems = [
  { label: "Overview", href: "/overview", icon: LayoutDashboard },
];

const settingsItems = [
  { label: "General", href: "/settings/general" },
  { label: "API Keys", href: "/settings/api-keys" },
];

interface SidebarProps {
  collapsed: boolean;
}

export function Sidebar({ collapsed }: SidebarProps) {
  const pathname = usePathname();

  return (
    <aside
      className={cn(
        "flex flex-col border-r bg-background transition-all duration-200",
        collapsed ? "w-[60px]" : "w-[240px]"
      )}
    >
      <nav className="flex flex-col gap-1 p-3">
        {navItems.map((item) => (
          <Link
            key={item.href}
            href={item.href}
            className={cn(
              "flex items-center gap-3 rounded-md px-3 py-2 text-sm transition-colors",
              "hover:bg-accent hover:text-accent-foreground",
              pathname === item.href &&
                "bg-accent font-semibold text-accent-foreground"
            )}
          >
            <item.icon className="h-4 w-4 shrink-0" />
            {!collapsed && <span>{item.label}</span>}
          </Link>
        ))}

        <div className="mt-4">
          <div
            className={cn(
              "flex items-center gap-3 px-3 py-2 text-sm text-muted-foreground",
              !collapsed && "mb-1"
            )}
          >
            <Settings className="h-4 w-4 shrink-0" />
            {!collapsed && <span>Settings</span>}
          </div>
          {!collapsed &&
            settingsItems.map((item) => (
              <Link
                key={item.href}
                href={item.href}
                className={cn(
                  "flex items-center rounded-md py-1.5 pl-10 pr-3 text-sm transition-colors",
                  "hover:bg-accent hover:text-accent-foreground",
                  pathname === item.href &&
                    "bg-accent font-semibold text-accent-foreground"
                )}
              >
                {item.label}
              </Link>
            ))}
        </div>
      </nav>
    </aside>
  );
}
