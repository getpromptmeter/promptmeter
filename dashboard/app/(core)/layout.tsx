"use client";

import { Suspense, useState } from "react";
import { Sidebar } from "@/components/layout/sidebar";
import { Header } from "@/components/layout/header";
import { PeriodProvider, usePeriod } from "@/lib/period-context";

function CoreLayoutInner({ children }: { children: React.ReactNode }) {
  const [sidebarCollapsed, setSidebarCollapsed] = useState(false);
  const { period, setPeriod } = usePeriod();

  return (
    <div className="flex h-screen flex-col">
      <Header
        period={period}
        onPeriodChange={setPeriod}
        onToggleSidebar={() => setSidebarCollapsed(!sidebarCollapsed)}
      />
      <div className="flex flex-1 overflow-hidden">
        <Sidebar collapsed={sidebarCollapsed} />
        <main className="flex-1 overflow-y-auto p-6">{children}</main>
      </div>
    </div>
  );
}

export default function CoreLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return (
    <Suspense fallback={<div className="flex h-screen items-center justify-center">Loading...</div>}>
      <PeriodProvider>
        <CoreLayoutInner>{children}</CoreLayoutInner>
      </PeriodProvider>
    </Suspense>
  );
}
