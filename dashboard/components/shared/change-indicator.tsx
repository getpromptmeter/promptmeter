import { cn } from "@/lib/utils";
import { ArrowUp, ArrowDown } from "lucide-react";

interface ChangeIndicatorProps {
  value: number | null;
  invertColor?: boolean;
  neutral?: boolean;
}

export function ChangeIndicator({
  value,
  invertColor = false,
  neutral = false,
}: ChangeIndicatorProps) {
  if (value === null || value === undefined) {
    return <span className="text-xs text-muted-foreground">--</span>;
  }

  const isPositive = value > 0;
  const isNegative = value < 0;

  let colorClass = "text-muted-foreground";
  if (!neutral) {
    if (invertColor) {
      // Inverted: positive = red (bad), negative = green (good)
      colorClass = isPositive
        ? "text-red-600"
        : isNegative
        ? "text-green-600"
        : "text-muted-foreground";
    } else {
      // Default: positive = red (cost up = bad), negative = green (cost down = good)
      colorClass = isPositive
        ? "text-red-600"
        : isNegative
        ? "text-green-600"
        : "text-muted-foreground";
    }
  }

  return (
    <span className={cn("inline-flex items-center gap-0.5 text-xs font-medium", colorClass)}>
      {isPositive ? (
        <ArrowUp className="h-3 w-3" />
      ) : isNegative ? (
        <ArrowDown className="h-3 w-3" />
      ) : null}
      {isPositive ? "+" : ""}
      {value.toFixed(1)}%
    </span>
  );
}
