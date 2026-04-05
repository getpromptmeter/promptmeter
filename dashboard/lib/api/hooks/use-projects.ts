"use client";

import { useQuery } from "@tanstack/react-query";
import { api } from "../client";
import type { Project } from "../types";

export function useProjects() {
  return useQuery({
    queryKey: ["projects"],
    queryFn: () => api.get<Project[]>("/api/v1/projects"),
    staleTime: 5 * 60 * 1000,
  });
}
