"use client";

import { useSearchParams } from "next/navigation";
import { useOverview } from "@/lib/api/hooks/use-overview";
import { useCost } from "@/lib/api/hooks/use-cost";
import { useCostTimeseries } from "@/lib/api/hooks/use-cost-timeseries";
import { KPICard } from "@/components/data/kpi-card";
import { CostBreakdownTable } from "@/components/data/cost-breakdown-table";
import { CostTimeseriesChart } from "@/components/charts/cost-timeseries-chart";
import { EmptyState } from "@/components/shared/empty-state";
import { formatCurrency, formatNumber, formatPercent } from "@/lib/utils/format";
import { Zap, Code } from "lucide-react";

export default function OverviewPage() {
  const searchParams = useSearchParams();
  const period = searchParams.get("period") || "7d";
  const project = searchParams.get("project") || undefined;

  const overview = useOverview({ period, project });
  const costByModel = useCost({ period, groupBy: "model", project });
  const costByFeature = useCost({ period, groupBy: "feature", project });
  const timeseries = useCostTimeseries({ period, project });

  const isLoading = overview.isLoading;
  const data = overview.data;

  // Welcome screen: no data yet
  if (!isLoading && (!data || data.total_requests === 0)) {
    return <WelcomeScreen />;
  }

  return (
    <div className="space-y-6">
      <h1 className="text-2xl font-bold tracking-tight">Overview</h1>

      {/* KPI Cards */}
      <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4">
        <KPICard
          label="Total Cost"
          value={data ? formatCurrency(data.total_cost) : "$0"}
          changePercent={data?.cost_change_percent ?? null}
          isLoading={isLoading}
        />
        <KPICard
          label="Total Requests"
          value={data ? formatNumber(data.total_requests) : "0"}
          changePercent={data?.requests_change_percent ?? null}
          neutral
          isLoading={isLoading}
        />
        <KPICard
          label="Error Rate"
          value={data ? formatPercent(data.error_rate) : "0%"}
          changePercent={data?.error_rate_change_percent ?? null}
          invertColor
          isLoading={isLoading}
        />
        <KPICard
          label="Avg Cost/Request"
          value={data ? formatCurrency(data.avg_cost_per_request) : "$0"}
          changePercent={data?.cost_per_req_change_percent ?? null}
          isLoading={isLoading}
        />
      </div>

      {/* Timeseries Chart */}
      <CostTimeseriesChart
        series={timeseries.data?.series ?? []}
        granularity={timeseries.data?.granularity ?? "day"}
        isLoading={timeseries.isLoading}
      />

      {/* Cost Breakdowns */}
      <div className="grid grid-cols-1 gap-6 lg:grid-cols-2">
        <CostBreakdownTable
          title="Cost by Model"
          items={costByModel.data ?? []}
          groupLabel="Model"
          isLoading={costByModel.isLoading}
        />
        <CostBreakdownTable
          title="Cost by Feature"
          items={costByFeature.data ?? []}
          groupLabel="Feature"
          isLoading={costByFeature.isLoading}
          emptyMessage={
            'No feature data yet. Add a "feature" tag in your SDK calls:\npm.track(model="gpt-4o", tags={"feature": "chatbot"})'
          }
        />
      </div>
    </div>
  );
}

function WelcomeScreen() {
  return (
    <div className="flex flex-col items-center justify-center gap-8 py-16">
      <div className="text-center">
        <h1 className="text-3xl font-bold tracking-tight">
          Know what your AI costs.
        </h1>
        <p className="mt-2 text-lg text-muted-foreground">
          Get started in under 2 minutes.
        </p>
      </div>

      <div className="grid grid-cols-1 gap-6 sm:grid-cols-2 max-w-2xl w-full">
        <EmptyState
          icon={Zap}
          title="Connect OpenAI"
          description="See your cost breakdown instantly -- no code."
          actionLabel="Connect OpenAI Account"
          actionHref="/settings/integrations"
        />
        <EmptyState
          icon={Code}
          title="Install SDK"
          description="2 lines of Python. Track costs per feature."
          actionLabel="View Setup Guide"
          actionHref="https://docs.promptmeter.dev/quickstart"
        />
      </div>
    </div>
  );
}
