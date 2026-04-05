"use client";

import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { formatCurrency, formatNumber } from "@/lib/utils/format";
import type { CostBreakdownItem } from "@/lib/api/types";

interface CostBreakdownTableProps {
  title: string;
  items: CostBreakdownItem[];
  groupLabel: string;
  isLoading?: boolean;
  emptyMessage?: string;
}

export function CostBreakdownTable({
  title,
  items,
  groupLabel,
  isLoading,
  emptyMessage,
}: CostBreakdownTableProps) {
  if (isLoading) {
    return (
      <Card>
        <CardHeader>
          <CardTitle className="text-base">{title}</CardTitle>
        </CardHeader>
        <CardContent>
          <div className="space-y-3">
            {[1, 2, 3].map((i) => (
              <Skeleton key={i} className="h-8 w-full" />
            ))}
          </div>
        </CardContent>
      </Card>
    );
  }

  if (!items || items.length === 0) {
    return (
      <Card>
        <CardHeader>
          <CardTitle className="text-base">{title}</CardTitle>
        </CardHeader>
        <CardContent>
          <p className="text-sm text-muted-foreground">
            {emptyMessage || "No data for this period."}
          </p>
        </CardContent>
      </Card>
    );
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-base">{title}</CardTitle>
      </CardHeader>
      <CardContent>
        <div className="space-y-3">
          <div className="grid grid-cols-[1fr_auto_auto_auto] gap-4 text-xs font-medium text-muted-foreground">
            <span>{groupLabel}</span>
            <span className="text-right">Cost</span>
            <span className="text-right">%</span>
            <span className="text-right">Requests</span>
          </div>
          {items.map((item) => (
            <div
              key={item.group}
              className="grid grid-cols-[1fr_auto_auto_auto] gap-4 items-center"
            >
              <div className="flex flex-col gap-1">
                <span className="text-sm font-medium truncate">
                  {item.group}
                </span>
                <div className="h-1.5 rounded-full bg-muted overflow-hidden">
                  <div
                    className="h-full rounded-full bg-primary"
                    style={{ width: `${Math.min(item.percent_of_total, 100)}%` }}
                  />
                </div>
              </div>
              <span className="text-sm font-medium text-right">
                {formatCurrency(item.cost_usd)}
              </span>
              <span className="text-sm text-muted-foreground text-right">
                {item.percent_of_total.toFixed(1)}%
              </span>
              <span className="text-sm text-muted-foreground text-right">
                {formatNumber(item.requests)}
              </span>
            </div>
          ))}
        </div>
      </CardContent>
    </Card>
  );
}
