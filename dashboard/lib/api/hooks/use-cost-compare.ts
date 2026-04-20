"use client";

import { useQuery } from "@tanstack/react-query";
import { api } from "../client";
import type { CostCompareResponse } from "../types";

export function useCostCompare(params: {
  currentFrom: string;
  currentTo: string;
  previousFrom: string;
  previousTo: string;
  groupBy?: string;
  project?: string;
  timezone?: string;
  model?: string;
  provider?: string;
  feature?: string;
  enabled?: boolean;
}) {
  return useQuery({
    queryKey: ["cost-compare", params],
    queryFn: () =>
      api.get<CostCompareResponse>("/api/v1/dashboard/cost/compare", {
        current_from: params.currentFrom,
        current_to: params.currentTo,
        previous_from: params.previousFrom,
        previous_to: params.previousTo,
        group_by: params.groupBy || "model",
        project: params.project || "",
        timezone: params.timezone || "",
        model: params.model || "",
        provider: params.provider || "",
        feature: params.feature || "",
      }),
    enabled: params.enabled !== false,
    staleTime: 60 * 1000,
    refetchInterval: 5 * 60 * 1000,
  });
}
