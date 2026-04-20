"use client";

import { Card, CardContent } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { ChangeIndicator } from "@/components/shared/change-indicator";
import { formatCurrency, formatNumber } from "@/lib/utils/format";
import type { CostCompareResponse } from "@/lib/api/types";

interface CostCompareCardsProps {
  data: CostCompareResponse | undefined;
  isLoading: boolean;
}

export function CostCompareCards({ data, isLoading }: CostCompareCardsProps) {
  if (isLoading) {
    return (
      <div className="grid grid-cols-1 gap-4 sm:grid-cols-3">
        {[1, 2, 3].map((i) => (
          <Card key={i}>
            <CardContent className="p-4">
              <Skeleton className="h-4 w-24 mb-2" />
              <Skeleton className="h-8 w-32" />
            </CardContent>
          </Card>
        ))}
      </div>
    );
  }

  if (!data) return null;

  return (
    <div className="grid grid-cols-1 gap-4 sm:grid-cols-3">
      <Card>
        <CardContent className="p-4">
          <p className="text-sm font-medium text-muted-foreground">
            This Period
          </p>
          <p className="mt-1 text-2xl font-bold tracking-tight">
            {formatCurrency(data.current.total_cost)}
          </p>
          <p className="mt-0.5 text-xs text-muted-foreground">
            {formatNumber(data.current.requests)} requests
          </p>
        </CardContent>
      </Card>

      <Card>
        <CardContent className="p-4">
          <p className="text-sm font-medium text-muted-foreground">
            Previous Period
          </p>
          <p className="mt-1 text-2xl font-bold tracking-tight">
            {formatCurrency(data.previous.total_cost)}
          </p>
          <p className="mt-0.5 text-xs text-muted-foreground">
            {formatNumber(data.previous.requests)} requests
          </p>
        </CardContent>
      </Card>

      <Card>
        <CardContent className="p-4">
          <p className="text-sm font-medium text-muted-foreground">Change</p>
          <p className="mt-1 text-2xl font-bold tracking-tight">
            {data.changes.cost_delta >= 0 ? "+" : ""}
            {formatCurrency(data.changes.cost_delta)}
          </p>
          <div className="mt-0.5">
            <ChangeIndicator value={data.changes.cost_percent} />
            {data.changes.request_percent !== null && (
              <span className="ml-2 text-xs text-muted-foreground">
                Requests: <ChangeIndicator value={data.changes.request_percent} neutral />
              </span>
            )}
          </div>
        </CardContent>
      </Card>
    </div>
  );
}
