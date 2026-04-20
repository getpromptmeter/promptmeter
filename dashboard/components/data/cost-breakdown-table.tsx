"use client";

import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { formatCurrency, formatNumber } from "@/lib/utils/format";
import { ChangeIndicator } from "@/components/shared/change-indicator";
import { ChevronRight, ChevronDown, ExternalLink } from "lucide-react";
import type { CostBreakdownItem, CostCompareBreakdownItem } from "@/lib/api/types";

interface CostBreakdownTableProps {
  title: string;
  items: CostBreakdownItem[];
  groupLabel: string;
  isLoading?: boolean;
  emptyMessage?: string;
  /** When set, clicking a row triggers drill-down. */
  onRowClick?: (group: string) => void;
  /** The currently expanded row (drill-down). */
  expandedGroup?: string | null;
  /** Drill-down child items for the expanded row. */
  drillDownItems?: CostBreakdownItem[];
  drillDownLoading?: boolean;
  drillDownLabel?: string;
  /** Compare mode: show compare columns instead of regular. */
  compareItems?: CostCompareBreakdownItem[];
  /** Navigate to events for a given row. */
  onViewEvents?: (group: string) => void;
}

export function CostBreakdownTable({
  title,
  items,
  groupLabel,
  isLoading,
  emptyMessage,
  onRowClick,
  expandedGroup,
  drillDownItems,
  drillDownLoading,
  drillDownLabel,
  compareItems,
  onViewEvents,
}: CostBreakdownTableProps) {
  if (isLoading) {
    return (
      <Card>
        <CardHeader>
          <CardTitle className="text-base">{title}</CardTitle>
        </CardHeader>
        <CardContent>
          <div className="space-y-3">
            {[1, 2, 3, 4, 5].map((i) => (
              <Skeleton key={i} className="h-8 w-full" />
            ))}
          </div>
        </CardContent>
      </Card>
    );
  }

  const displayItems = items ?? [];
  const hasData = compareItems ? compareItems.length > 0 : displayItems.length > 0;

  if (!hasData) {
    return (
      <Card>
        <CardHeader>
          <CardTitle className="text-base">{title}</CardTitle>
        </CardHeader>
        <CardContent>
          <p className="text-sm text-muted-foreground">
            {emptyMessage || "No data for this period."}
          </p>
        </CardContent>
      </Card>
    );
  }

  // Compare mode
  if (compareItems) {
    return (
      <Card>
        <CardHeader>
          <CardTitle className="text-base">{title}</CardTitle>
        </CardHeader>
        <CardContent>
          <div className="space-y-3">
            <div className="grid grid-cols-[1fr_auto_auto_auto_auto] gap-4 text-xs font-medium text-muted-foreground">
              <span>{groupLabel}</span>
              <span className="text-right">Current</span>
              <span className="text-right">Previous</span>
              <span className="text-right">Change</span>
              <span className="text-right">Requests</span>
            </div>
            {compareItems.map((item) => (
              <div
                key={item.group}
                className="grid grid-cols-[1fr_auto_auto_auto_auto] gap-4 items-center"
              >
                <span className="text-sm font-medium truncate">
                  {item.group}
                </span>
                <span className="text-sm font-medium text-right">
                  {formatCurrency(item.current_cost)}
                </span>
                <span className="text-sm text-muted-foreground text-right">
                  {formatCurrency(item.previous_cost)}
                </span>
                <span className="text-right">
                  <ChangeIndicator value={item.cost_change} />
                </span>
                <span className="text-sm text-muted-foreground text-right">
                  {formatNumber(item.requests)}
                </span>
              </div>
            ))}
          </div>
        </CardContent>
      </Card>
    );
  }

  // Regular mode with drill-down support
  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-base">{title}</CardTitle>
      </CardHeader>
      <CardContent>
        <div className="space-y-1">
          <div className="grid grid-cols-[auto_1fr_auto_auto_auto_auto] gap-4 text-xs font-medium text-muted-foreground px-2 pb-2">
            <span className="w-5" />
            <span>{groupLabel}</span>
            <span className="text-right">Cost</span>
            <span className="text-right">%</span>
            <span className="text-right">Requests</span>
            <span className="w-5" />
          </div>
          {displayItems.map((item) => {
            const isExpanded = expandedGroup === item.group;
            return (
              <div key={item.group}>
                <div
                  className={`grid grid-cols-[auto_1fr_auto_auto_auto_auto] gap-4 items-center rounded-md px-2 py-2 ${
                    onRowClick
                      ? "cursor-pointer hover:bg-muted/50 transition-colors"
                      : ""
                  } ${isExpanded ? "bg-muted/30" : ""}`}
                  onClick={() => onRowClick?.(item.group)}
                >
                  <span className="w-5 flex-shrink-0 text-muted-foreground">
                    {onRowClick &&
                      (isExpanded ? (
                        <ChevronDown className="h-4 w-4" />
                      ) : (
                        <ChevronRight className="h-4 w-4" />
                      ))}
                  </span>
                  <div className="flex flex-col gap-1 min-w-0">
                    <span className="text-sm font-medium truncate">
                      {item.group}
                    </span>
                    <div className="h-1.5 rounded-full bg-muted overflow-hidden">
                      <div
                        className="h-full rounded-full bg-primary"
                        style={{
                          width: `${Math.min(item.percent_of_total, 100)}%`,
                        }}
                      />
                    </div>
                  </div>
                  <span className="text-sm font-medium text-right">
                    {formatCurrency(item.cost_usd)}
                  </span>
                  <span className="text-sm text-muted-foreground text-right">
                    {item.percent_of_total.toFixed(1)}%
                  </span>
                  <span className="text-sm text-muted-foreground text-right">
                    {formatNumber(item.requests)}
                  </span>
                  <span className="w-5 flex-shrink-0">
                    {onViewEvents && (
                      <button
                        className="text-muted-foreground hover:text-foreground transition-colors"
                        title="View events"
                        onClick={(e) => {
                          e.stopPropagation();
                          onViewEvents(item.group);
                        }}
                      >
                        <ExternalLink className="h-3.5 w-3.5" />
                      </button>
                    )}
                  </span>
                </div>

                {/* Drill-down expanded content */}
                {isExpanded && (
                  <div className="pl-9 pr-2 py-1">
                    {drillDownLoading ? (
                      <div className="space-y-2 py-2">
                        {[1, 2, 3].map((i) => (
                          <Skeleton key={i} className="h-6 w-full" />
                        ))}
                      </div>
                    ) : drillDownItems && drillDownItems.length > 0 ? (
                      <div className="space-y-1">
                        <div className="grid grid-cols-[1fr_auto_auto_auto] gap-4 text-xs font-medium text-muted-foreground pb-1">
                          <span>{drillDownLabel || "Sub-group"}</span>
                          <span className="text-right">Cost</span>
                          <span className="text-right">%</span>
                          <span className="text-right">Requests</span>
                        </div>
                        {drillDownItems.map((sub) => (
                          <div
                            key={sub.group}
                            className="grid grid-cols-[1fr_auto_auto_auto] gap-4 items-center py-1 text-sm"
                          >
                            <span className="text-muted-foreground truncate">
                              {sub.group}
                            </span>
                            <span className="text-right">
                              {formatCurrency(sub.cost_usd)}
                            </span>
                            <span className="text-right text-muted-foreground">
                              {sub.percent_of_total.toFixed(1)}%
                            </span>
                            <span className="text-right text-muted-foreground">
                              {formatNumber(sub.requests)}
                            </span>
                          </div>
                        ))}
                      </div>
                    ) : (
                      <p className="text-xs text-muted-foreground py-2">
                        No breakdown data available.
                      </p>
                    )}
                  </div>
                )}
              </div>
            );
          })}
        </div>
      </CardContent>
    </Card>
  );
}
