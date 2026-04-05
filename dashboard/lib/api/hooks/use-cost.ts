"use client";

import { useQuery } from "@tanstack/react-query";
import { api } from "../client";
import type { CostBreakdownItem } from "../types";

export function useCost(params: {
  period: string;
  groupBy: string;
  project?: string;
  timezone?: string;
}) {
  return useQuery({
    queryKey: ["cost", params],
    queryFn: () =>
      api.get<CostBreakdownItem[]>("/api/v1/dashboard/cost", {
        period: params.period,
        group_by: params.groupBy,
        project: params.project || "",
        timezone: params.timezone || "",
      }),
    staleTime: 60 * 1000,
    refetchInterval: 5 * 60 * 1000,
  });
}
