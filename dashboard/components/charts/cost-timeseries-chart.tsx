"use client";

import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { AreaChart } from "@tremor/react";
import { format } from "date-fns";
import { formatCurrency } from "@/lib/utils/format";
import type { TimeseriesSeries } from "@/lib/api/types";

interface CostTimeseriesChartProps {
  series: TimeseriesSeries[];
  granularity: "hour" | "day" | "week";
  isLoading?: boolean;
  groupBy?: string;
  onGroupByChange?: (groupBy: string) => void;
}

const dateFormatForGranularity = {
  hour: "HH:mm",
  day: "MMM dd",
  week: "MMM dd",
};

export function CostTimeseriesChart({
  series,
  granularity,
  isLoading,
}: CostTimeseriesChartProps) {
  if (isLoading) {
    return (
      <Card>
        <CardHeader>
          <CardTitle className="text-base">Cost Over Time</CardTitle>
        </CardHeader>
        <CardContent>
          <Skeleton className="h-[300px] w-full" />
        </CardContent>
      </Card>
    );
  }

  if (!series || series.length === 0 || series.every((s) => s.points.length === 0)) {
    return (
      <Card>
        <CardHeader>
          <CardTitle className="text-base">Cost Over Time</CardTitle>
        </CardHeader>
        <CardContent>
          <div className="flex h-[300px] items-center justify-center text-sm text-muted-foreground">
            No data for this period
          </div>
        </CardContent>
      </Card>
    );
  }

  // Transform series data into Tremor AreaChart format
  const dateFormat = dateFormatForGranularity[granularity];
  const categories = series.map((s) => s.group === "_total" ? "Total Cost" : s.group);

  // Build chart data: array of objects with date + one key per series
  const dateMap = new Map<string, Record<string, number | string>>();

  for (const s of series) {
    const seriesName = s.group === "_total" ? "Total Cost" : s.group;
    for (const pt of s.points) {
      const dateKey = format(new Date(pt.timestamp), dateFormat);
      if (!dateMap.has(dateKey)) {
        dateMap.set(dateKey, { date: dateKey });
      }
      const row = dateMap.get(dateKey)!;
      row[seriesName] = (row[seriesName] as number || 0) + pt.cost_usd;
    }
  }

  const chartData = Array.from(dateMap.values());

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-base">Cost Over Time</CardTitle>
      </CardHeader>
      <CardContent>
        <AreaChart
          className="h-[300px]"
          data={chartData}
          index="date"
          categories={categories}
          valueFormatter={formatCurrency}
          showLegend={categories.length > 1}
          curveType="monotone"
        />
      </CardContent>
    </Card>
  );
}
