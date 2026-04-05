"use client";

import { Menu } from "lucide-react";
import { Button } from "@/components/ui/button";
import { PeriodSelector } from "./period-selector";

interface HeaderProps {
  period: string;
  onPeriodChange: (period: string) => void;
  onToggleSidebar: () => void;
}

export function Header({ period, onPeriodChange, onToggleSidebar }: HeaderProps) {
  return (
    <header className="flex h-14 items-center justify-between border-b bg-background px-4">
      <div className="flex items-center gap-3">
        <Button
          variant="ghost"
          size="icon"
          className="h-8 w-8"
          onClick={onToggleSidebar}
        >
          <Menu className="h-4 w-4" />
        </Button>
        <span className="text-lg font-semibold">Promptmeter</span>
      </div>

      <div className="flex items-center gap-4">
        <PeriodSelector value={period} onChange={onPeriodChange} />
      </div>
    </header>
  );
}
