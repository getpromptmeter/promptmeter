"use client";

import { useQuery } from "@tanstack/react-query";
import { api } from "../client";
import type { OverviewResponse } from "../types";

export function useOverview(params: {
  period: string;
  project?: string;
  timezone?: string;
}) {
  return useQuery({
    queryKey: ["overview", params],
    queryFn: () =>
      api.get<OverviewResponse>("/api/v1/dashboard/overview", {
        period: params.period,
        project: params.project || "",
        timezone: params.timezone || "",
      }),
    staleTime: 60 * 1000,
    refetchInterval: 5 * 60 * 1000,
  });
}
