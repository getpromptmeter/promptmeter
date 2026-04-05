"use client";

import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { api } from "../client";
import type { OrgSettings, UpdateOrgSettingsRequest } from "../types";

export function useSettings() {
  return useQuery({
    queryKey: ["settings"],
    queryFn: () => api.get<OrgSettings>("/api/v1/settings/org"),
    staleTime: 5 * 60 * 1000,
  });
}

export function useUpdateSettings() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (req: UpdateOrgSettingsRequest) =>
      api.put<OrgSettings>("/api/v1/settings/org", req),
    onSuccess: (data) => {
      queryClient.setQueryData(["settings"], data);
    },
  });
}
