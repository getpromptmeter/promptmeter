"use client";

import { useSearchParams, useRouter } from "next/navigation";
import { useQueryState, parseAsBoolean } from "nuqs";
import { usePeriod } from "@/lib/period-context";
import { useCost } from "@/lib/api/hooks/use-cost";
import { useCostTimeseries } from "@/lib/api/hooks/use-cost-timeseries";
import { useCostCompare } from "@/lib/api/hooks/use-cost-compare";
import { useCostFilters } from "@/lib/api/hooks/use-cost-filters";
import { CostFilterBar } from "@/components/data/cost-filter-bar";
import { CostCompareCards } from "@/components/data/cost-compare-cards";
import { CostBreakdownTable } from "@/components/data/cost-breakdown-table";
import { CostTimeseriesChart } from "@/components/charts/cost-timeseries-chart";
import { Button } from "@/components/ui/button";
import { downloadCSV } from "@/lib/utils/csv";
import { ArrowLeftRight, Download } from "lucide-react";
import { useState, useMemo } from "react";

// Period duration in ms for computing previous period
const PERIOD_DURATIONS: Record<string, number> = {
  "1d": 24 * 60 * 60 * 1000,
  "7d": 7 * 24 * 60 * 60 * 1000,
  "30d": 30 * 24 * 60 * 60 * 1000,
  "90d": 90 * 24 * 60 * 60 * 1000,
};

function getPreviousPeriod(period: string) {
  const now = new Date();
  const durationMs = PERIOD_DURATIONS[period] ?? PERIOD_DURATIONS["7d"];
  const currentTo = now;
  const currentFrom = new Date(now.getTime() - durationMs);
  const previousTo = new Date(currentFrom.getTime());
  const previousFrom = new Date(currentFrom.getTime() - durationMs);
  return {
    currentFrom: currentFrom.toISOString(),
    currentTo: currentTo.toISOString(),
    previousFrom: previousFrom.toISOString(),
    previousTo: previousTo.toISOString(),
  };
}

export default function CostExplorerPage() {
  const router = useRouter();
  const searchParams = useSearchParams();
  const { period } = usePeriod();
  const project = searchParams.get("project") || undefined;

  // Page-specific URL state via nuqs
  const [groupBy, setGroupBy] = useQueryState("group_by", {
    defaultValue: "model",
  });
  const [modelFilter, setModelFilter] = useQueryState("model");
  const [providerFilter, setProviderFilter] = useQueryState("provider");
  const [featureFilter, setFeatureFilter] = useQueryState("feature");
  const [compare, setCompare] = useQueryState(
    "compare",
    parseAsBoolean.withDefault(false)
  );

  // Drill-down state (local, not in URL)
  const [expandedGroup, setExpandedGroup] = useState<string | null>(null);

  // Filter values for dropdowns
  const filters = useCostFilters({ period, project });

  // Main breakdown query
  const breakdown = useCost({
    period,
    groupBy,
    project,
    model: modelFilter || undefined,
    provider: providerFilter || undefined,
    feature: featureFilter || undefined,
  });

  // Timeseries query (no split by project)
  const timeseriesGroupBy = groupBy === "project" ? undefined : groupBy;
  const timeseries = useCostTimeseries({
    period,
    groupBy: timeseriesGroupBy,
    project,
    model: modelFilter || undefined,
    provider: providerFilter || undefined,
    feature: featureFilter || undefined,
  });

  // Compare query (only when compare mode is on)
  const comparePeriods = useMemo(() => getPreviousPeriod(period), [period]);
  const compareData = useCostCompare({
    ...comparePeriods,
    groupBy,
    project,
    model: modelFilter || undefined,
    provider: providerFilter || undefined,
    feature: featureFilter || undefined,
    enabled: compare,
  });

  // Drill-down query: when a row is expanded, fetch the cross-dimensional breakdown
  const drillDownGroupBy = groupBy === "model" ? "feature" : "model";
  const drillDownFilterKey = groupBy === "model" ? "model" : "feature";
  const drillDown = useCost({
    period,
    groupBy: drillDownGroupBy,
    project,
    model:
      drillDownFilterKey === "model"
        ? expandedGroup || undefined
        : modelFilter || undefined,
    provider: providerFilter || undefined,
    feature:
      drillDownFilterKey === "feature"
        ? expandedGroup || undefined
        : featureFilter || undefined,
  });

  const handleRowClick = (group: string) => {
    if (groupBy === "project") {
      // Switch project selector to clicked project, change group_by to model
      const params = new URLSearchParams(searchParams.toString());
      params.set("project", group);
      params.set("group_by", "model");
      router.push(`/cost?${params.toString()}`);
      return;
    }
    setExpandedGroup((prev) => (prev === group ? null : group));
  };

  const handleViewEvents = (group: string) => {
    const params = new URLSearchParams();
    if (period) params.set("period", period);
    if (project) params.set("project", project);
    if (groupBy === "model") {
      params.set("model", group);
    } else if (groupBy === "feature") {
      params.set("feature", group);
    }
    if (modelFilter) params.set("model", modelFilter);
    if (providerFilter) params.set("provider", providerFilter);
    if (featureFilter) params.set("feature", featureFilter);
    router.push(`/events?${params.toString()}`);
  };

  const handleExportCSV = () => {
    if (breakdown.data) {
      downloadCSV(breakdown.data, `cost-${groupBy}-${period}.csv`);
    }
  };

  const groupLabel =
    groupBy === "model"
      ? "Model"
      : groupBy === "feature"
        ? "Feature"
        : "Project";

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-bold tracking-tight">Cost Explorer</h1>
        <div className="flex items-center gap-2">
          <Button
            variant={compare ? "default" : "outline"}
            size="sm"
            onClick={() => setCompare(!compare)}
          >
            <ArrowLeftRight className="mr-1.5 h-4 w-4" />
            Compare
          </Button>
          <Button
            variant="outline"
            size="sm"
            onClick={handleExportCSV}
            disabled={!breakdown.data || breakdown.data.length === 0}
          >
            <Download className="mr-1.5 h-4 w-4" />
            CSV
          </Button>
        </div>
      </div>

      {/* Control Bar */}
      <CostFilterBar
        groupBy={groupBy}
        onGroupByChange={(v) => {
          setGroupBy(v);
          setExpandedGroup(null);
        }}
        modelFilter={modelFilter}
        onModelFilterChange={setModelFilter}
        providerFilter={providerFilter}
        onProviderFilterChange={setProviderFilter}
        featureFilter={featureFilter}
        onFeatureFilterChange={setFeatureFilter}
        models={filters.data?.models ?? []}
        providers={filters.data?.providers ?? []}
        features={filters.data?.features ?? []}
        isProjectSelected={!!project}
        isLoading={filters.isLoading}
      />

      {/* Compare Cards */}
      {compare && (
        <CostCompareCards
          data={compareData.data}
          isLoading={compareData.isLoading}
        />
      )}

      {/* Timeseries Chart */}
      <CostTimeseriesChart
        series={timeseries.data?.series ?? []}
        granularity={timeseries.data?.granularity ?? "day"}
        isLoading={timeseries.isLoading}
      />

      {/* Breakdown Table */}
      {compare && compareData.data ? (
        <CostBreakdownTable
          title={`Cost by ${groupLabel}`}
          items={[]}
          groupLabel={groupLabel}
          isLoading={compareData.isLoading}
          compareItems={compareData.data.breakdown}
          emptyMessage="No cost data for the selected filters. Try adjusting the time period or removing some filters."
        />
      ) : (
        <CostBreakdownTable
          title={`Cost by ${groupLabel}`}
          items={breakdown.data ?? []}
          groupLabel={groupLabel}
          isLoading={breakdown.isLoading}
          onRowClick={groupBy !== "project" ? handleRowClick : handleRowClick}
          expandedGroup={expandedGroup}
          drillDownItems={drillDown.data ?? []}
          drillDownLoading={drillDown.isLoading && expandedGroup !== null}
          drillDownLabel={
            groupBy === "model" ? "Feature" : "Model"
          }
          onViewEvents={handleViewEvents}
          emptyMessage="No cost data for the selected filters. Try adjusting the time period or removing some filters."
        />
      )}
    </div>
  );
}
