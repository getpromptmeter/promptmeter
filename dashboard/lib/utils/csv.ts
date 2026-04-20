import type { CostBreakdownItem } from "@/lib/api/types";

/**
 * Generate and download a CSV file from cost breakdown items.
 */
export function downloadCSV(
  items: CostBreakdownItem[],
  filename: string = "cost-breakdown.csv"
) {
  const headers = ["Group", "Cost (USD)", "% of Total", "Requests", "Tokens"];
  const rows = items.map((item) => [
    escapeCSV(item.group),
    item.cost_usd.toFixed(4),
    item.percent_of_total.toFixed(1),
    String(item.requests),
    String(item.tokens ?? ""),
  ]);

  const csv = [headers.join(","), ...rows.map((r) => r.join(","))].join("\n");

  const blob = new Blob([csv], { type: "text/csv;charset=utf-8;" });
  const url = URL.createObjectURL(blob);
  const link = document.createElement("a");
  link.href = url;
  link.download = filename;
  link.click();
  URL.revokeObjectURL(url);
}

function escapeCSV(value: string): string {
  if (value.includes(",") || value.includes('"') || value.includes("\n")) {
    return `"${value.replace(/"/g, '""')}"`;
  }
  return value;
}
