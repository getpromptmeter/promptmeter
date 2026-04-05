"use client";

import { cn } from "@/lib/utils";

const periods = ["1d", "7d", "30d", "90d"] as const;
type Period = (typeof periods)[number];

interface PeriodSelectorProps {
  value: string;
  onChange: (period: string) => void;
}

export function PeriodSelector({ value, onChange }: PeriodSelectorProps) {
  return (
    <div className="flex items-center gap-1 rounded-md border p-0.5">
      {periods.map((period) => (
        <button
          key={period}
          onClick={() => onChange(period)}
          className={cn(
            "rounded px-3 py-1 text-xs font-medium transition-colors",
            value === period
              ? "bg-primary text-primary-foreground"
              : "text-muted-foreground hover:text-foreground"
          )}
        >
          {period.toUpperCase()}
        </button>
      ))}
    </div>
  );
}
