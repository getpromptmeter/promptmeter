"use client";

import { Tabs, TabsList, TabsTrigger } from "@/components/ui/tabs";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Button } from "@/components/ui/button";
import { X } from "lucide-react";

interface CostFilterBarProps {
  groupBy: string;
  onGroupByChange: (value: string) => void;
  modelFilter: string | null;
  onModelFilterChange: (value: string | null) => void;
  providerFilter: string | null;
  onProviderFilterChange: (value: string | null) => void;
  featureFilter: string | null;
  onFeatureFilterChange: (value: string | null) => void;
  models: string[];
  providers: string[];
  features: string[];
  isProjectSelected: boolean;
  isLoading?: boolean;
}

export function CostFilterBar({
  groupBy,
  onGroupByChange,
  modelFilter,
  onModelFilterChange,
  providerFilter,
  onProviderFilterChange,
  featureFilter,
  onFeatureFilterChange,
  models,
  providers,
  features,
  isProjectSelected,
  isLoading,
}: CostFilterBarProps) {
  const hasActiveFilters = modelFilter || providerFilter || featureFilter;

  const clearAll = () => {
    onModelFilterChange(null);
    onProviderFilterChange(null);
    onFeatureFilterChange(null);
  };

  return (
    <div className="space-y-3">
      <div className="flex items-center gap-4">
        <span className="text-sm font-medium text-muted-foreground">
          Group by
        </span>
        <Tabs value={groupBy} onValueChange={onGroupByChange}>
          <TabsList>
            <TabsTrigger value="model">Model</TabsTrigger>
            <TabsTrigger value="feature">Feature</TabsTrigger>
            <TabsTrigger value="project" disabled={isProjectSelected}>
              Project
            </TabsTrigger>
          </TabsList>
        </Tabs>
      </div>

      <div className="flex flex-wrap items-center gap-3">
        <FilterSelect
          label="Model"
          value={modelFilter}
          onChange={onModelFilterChange}
          options={models}
          disabled={isLoading}
        />
        <FilterSelect
          label="Provider"
          value={providerFilter}
          onChange={onProviderFilterChange}
          options={providers}
          disabled={isLoading}
        />
        <FilterSelect
          label="Feature"
          value={featureFilter}
          onChange={onFeatureFilterChange}
          options={features}
          disabled={isLoading}
        />
        {hasActiveFilters && (
          <Button variant="ghost" size="sm" onClick={clearAll} className="h-8">
            <X className="mr-1 h-3 w-3" />
            Clear all
          </Button>
        )}
      </div>
    </div>
  );
}

function FilterSelect({
  label,
  value,
  onChange,
  options,
  disabled,
}: {
  label: string;
  value: string | null;
  onChange: (value: string | null) => void;
  options: string[];
  disabled?: boolean;
}) {
  return (
    <Select
      value={value ?? ""}
      onValueChange={(val) => onChange(val === "" ? null : val)}
      disabled={disabled}
    >
      <SelectTrigger size="sm" className="min-w-[120px]">
        <SelectValue placeholder={`${label}: All`} />
      </SelectTrigger>
      <SelectContent>
        <SelectItem value="">All {label}s</SelectItem>
        {options.map((opt) => (
          <SelectItem key={opt} value={opt}>
            {opt}
          </SelectItem>
        ))}
      </SelectContent>
    </Select>
  );
}
