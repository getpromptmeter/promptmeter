"use client";

import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import {
  ChartContainer,
  ChartTooltipContent,
} from "@/components/ui/chart";
import { AreaChart, Area, XAxis, YAxis, CartesianGrid, Tooltip, ResponsiveContainer } from "recharts";
import { format } from "date-fns";
import { formatCurrency } from "@/lib/utils/format";
import { chartConfigForCategories } from "@/lib/chart-colors";
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

  // Transform series data into Recharts format
  const dateFormat = dateFormatForGranularity[granularity];
  const categories = series.map((s) => {
    if (s.group === "_total") return "Total Cost";
    if (s.group === "_other") return "Other";
    return s.group;
  });

  // Build chart data: array of objects with date + one key per series
  const dateMap = new Map<string, Record<string, number | string>>();

  for (const s of series) {
    const seriesName = s.group === "_total" ? "Total Cost" : s.group === "_other" ? "Other" : s.group;
    for (const pt of s.points) {
      const dateKey = format(new Date(pt.timestamp), dateFormat);
      if (!dateMap.has(dateKey)) {
        dateMap.set(dateKey, { date: dateKey });
      }
      const row = dateMap.get(dateKey)!;
      row[seriesName] = ((row[seriesName] as number) || 0) + pt.cost_usd;
    }
  }

  const chartData = Array.from(dateMap.values());
  const { config, colors } = chartConfigForCategories(categories);
  const isStacked = categories.length > 1;

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-base">Cost Over Time</CardTitle>
      </CardHeader>
      <CardContent>
        <ChartContainer config={config} className="h-[300px] w-full">
          <ResponsiveContainer width="100%" height="100%">
            <AreaChart data={chartData} margin={{ top: 4, right: 4, bottom: 0, left: 0 }}>
              <CartesianGrid vertical={false} strokeDasharray="3 3" className="stroke-border" />
              <XAxis
                dataKey="date"
                tickLine={false}
                axisLine={false}
                tickMargin={8}
                className="text-xs fill-muted-foreground"
              />
              <YAxis
                tickLine={false}
                axisLine={false}
                tickMargin={8}
                tickFormatter={formatCurrency}
                className="text-xs fill-muted-foreground"
                width={60}
              />
              <Tooltip
                content={<ChartTooltipContent formatter={formatCurrency} />}
              />
              {categories.map((cat) => (
                <Area
                  key={cat}
                  type="monotone"
                  dataKey={cat}
                  stackId={isStacked ? "cost" : undefined}
                  fill={colors[cat]}
                  stroke={colors[cat]}
                  fillOpacity={0.3}
                  strokeWidth={2}
                />
              ))}
            </AreaChart>
          </ResponsiveContainer>
        </ChartContainer>
        {isStacked && (
          <div className="mt-3 flex flex-wrap items-center justify-center gap-x-4 gap-y-1">
            {categories.map((cat) => (
              <div key={cat} className="flex items-center gap-1.5 text-xs text-muted-foreground">
                <span
                  className="inline-block h-2.5 w-2.5 rounded-full"
                  style={{ backgroundColor: colors[cat] }}
                />
                {cat}
              </div>
            ))}
          </div>
        )}
      </CardContent>
    </Card>
  );
}
