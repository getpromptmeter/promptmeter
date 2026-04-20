// Chart color palette using HSL values (Recharts-compatible).
// Each model/series gets a distinct, readable color.

import type { ChartConfig } from "@/components/ui/chart";

export const CHART_PALETTE: Record<string, string> = {
  "claude-3-5-sonnet": "hsl(239, 84%, 67%)", // indigo
  "claude-3-5-haiku": "hsl(350, 89%, 60%)", // rose
  "gpt-4o": "hsl(160, 84%, 39%)", // emerald
  "gpt-4o-mini": "hsl(258, 90%, 66%)", // violet
  "gpt-4-turbo": "hsl(38, 92%, 50%)", // amber
  "gemini-1.5-pro": "hsl(187, 92%, 41%)", // cyan
  "gemini-1.5-flash": "hsl(172, 66%, 40%)", // teal
  _other: "hsl(25, 95%, 53%)", // orange
  Other: "hsl(25, 95%, 53%)", // orange
  _total: "hsl(239, 84%, 67%)", // indigo
  "Total Cost": "hsl(239, 84%, 67%)", // indigo
};

// Fallback palette for unknown models
export const CHART_COLORS = [
  "hsl(239, 84%, 67%)", // indigo
  "hsl(160, 84%, 39%)", // emerald
  "hsl(38, 92%, 50%)", // amber
  "hsl(187, 92%, 41%)", // cyan
  "hsl(350, 89%, 60%)", // rose
  "hsl(258, 90%, 66%)", // violet
  "hsl(172, 66%, 40%)", // teal
  "hsl(25, 95%, 53%)", // orange
];

/**
 * Build a Recharts-compatible ChartConfig + color map from an array of category names.
 * Returns { config, colors } where colors[category] = hsl string.
 */
export function chartConfigForCategories(categories: string[]): {
  config: ChartConfig;
  colors: Record<string, string>;
} {
  const config: ChartConfig = {};
  const colors: Record<string, string> = {};
  const usedColors = new Set<string>();

  for (const cat of categories) {
    const mapped = CHART_PALETTE[cat];
    if (mapped && !usedColors.has(mapped)) {
      usedColors.add(mapped);
      colors[cat] = mapped;
    } else {
      // Fallback: pick next unused color from palette
      const fallback = CHART_COLORS.find((c) => !usedColors.has(c));
      const color = fallback ?? "hsl(220, 9%, 46%)"; // gray fallback
      usedColors.add(color);
      colors[cat] = color;
    }

    config[cat] = {
      label: cat,
      color: colors[cat],
    };
  }

  return { config, colors };
}
