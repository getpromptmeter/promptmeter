"use client";

import * as React from "react";
import { cn } from "@/lib/utils";

// Chart config type used by ChartContainer to provide colors/labels
export type ChartConfig = Record<
  string,
  {
    label?: string;
    color?: string;
  }
>;

type ChartContextProps = {
  config: ChartConfig;
};

const ChartContext = React.createContext<ChartContextProps | null>(null);

function useChart() {
  const context = React.useContext(ChartContext);
  if (!context) {
    throw new Error("useChart must be used within a <ChartContainer />");
  }
  return context;
}

interface ChartContainerProps extends React.HTMLAttributes<HTMLDivElement> {
  config: ChartConfig;
  children: React.ReactNode;
}

const ChartContainer = React.forwardRef<HTMLDivElement, ChartContainerProps>(
  ({ config, children, className, ...props }, ref) => {
    // Generate CSS variables for each config entry
    const style = Object.entries(config).reduce<Record<string, string>>(
      (acc, [key, value]) => {
        if (value.color) {
          acc[`--color-${key}`] = value.color;
        }
        return acc;
      },
      {}
    );

    return (
      <ChartContext.Provider value={{ config }}>
        <div
          ref={ref}
          className={cn("flex aspect-auto justify-center text-xs", className)}
          style={style}
          {...props}
        >
          {children}
        </div>
      </ChartContext.Provider>
    );
  }
);
ChartContainer.displayName = "ChartContainer";

// Tooltip
interface ChartTooltipContentProps {
  active?: boolean;
  payload?: Array<{ name: string; value: number; color: string }>;
  label?: string;
  hideLabel?: boolean;
  formatter?: (value: number) => string;
  labelFormatter?: (label: string) => string;
}

function ChartTooltipContent({
  active,
  payload,
  label,
  hideLabel = false,
  formatter,
  labelFormatter,
}: ChartTooltipContentProps) {
  if (!active || !payload?.length) {
    return null;
  }

  return (
    <div className="rounded-lg border border-border bg-card px-3 py-2 shadow-md">
      {!hideLabel && (
        <div className="mb-1.5 text-xs font-medium text-muted-foreground">
          {labelFormatter && label ? labelFormatter(label) : label}
        </div>
      )}
      <div className="flex flex-col gap-1">
        {payload.map(
          (
            entry,
            index: number
          ) => (
            <div key={index} className="flex items-center gap-2 text-xs">
              <span
                className="inline-block h-2.5 w-2.5 shrink-0 rounded-full"
                style={{ backgroundColor: entry.color }}
              />
              <span className="flex-1 text-muted-foreground">{entry.name}</span>
              <span className="font-mono font-medium text-foreground">
                {formatter ? formatter(entry.value) : entry.value}
              </span>
            </div>
          )
        )}
      </div>
    </div>
  );
}

export {
  ChartContainer,
  ChartTooltipContent,
  useChart,
};
