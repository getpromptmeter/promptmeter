/**
 * Format currency values.
 * Small amounts (< $1): $0.0034 (4 decimal places)
 * Normal amounts: $1,234 (no decimals)
 * Large amounts: $1.2M (abbreviated)
 */
export function formatCurrency(value: number): string {
  if (value === 0) return "$0";

  if (Math.abs(value) >= 1_000_000) {
    return "$" + (value / 1_000_000).toFixed(1) + "M";
  }
  if (Math.abs(value) >= 1_000) {
    return "$" + value.toLocaleString("en-US", { maximumFractionDigits: 0 });
  }
  if (Math.abs(value) >= 1) {
    return "$" + value.toFixed(2);
  }
  // Small amounts
  return "$" + value.toFixed(4);
}

/**
 * Format numbers with K/M suffixes.
 * 1,250 -> 1.2K
 * 1,500,000 -> 1.5M
 */
export function formatNumber(value: number): string {
  if (value === 0) return "0";

  if (Math.abs(value) >= 1_000_000) {
    return (value / 1_000_000).toFixed(1) + "M";
  }
  if (Math.abs(value) >= 10_000) {
    return (value / 1_000).toFixed(1) + "K";
  }
  return value.toLocaleString("en-US");
}

/**
 * Format a percentage value.
 */
export function formatPercent(value: number): string {
  return value.toFixed(1) + "%";
}

/**
 * Format duration in milliseconds.
 */
export function formatDuration(ms: number): string {
  if (ms < 1000) return ms.toFixed(0) + "ms";
  return (ms / 1000).toFixed(1) + "s";
}
