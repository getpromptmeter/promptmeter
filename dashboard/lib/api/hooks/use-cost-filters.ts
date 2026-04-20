"use client";

import { useQuery } from "@tanstack/react-query";
import { api } from "../client";
import type { CostFiltersResponse } from "../types";

export function useCostFilters(params: {
  period: string;
  project?: string;
  timezone?: string;
}) {
  return useQuery({
    queryKey: ["cost-filters", params],
    queryFn: () =>
      api.get<CostFiltersResponse>("/api/v1/dashboard/cost/filters", {
        period: params.period,
        project: params.project || "",
        timezone: params.timezone || "",
      }),
    staleTime: 5 * 60 * 1000,
    refetchInterval: 5 * 60 * 1000,
  });
}
