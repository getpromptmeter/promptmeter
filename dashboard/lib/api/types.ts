// Standard API response envelope
export interface ApiResponse<T> {
  data: T;
  meta: {
    request_id: string;
    timestamp: string;
    [key: string]: unknown;
  };
}

export interface ApiError {
  error: {
    code: string;
    message: string;
    details?: Record<string, unknown>;
  };
  meta: {
    request_id?: string;
    timestamp?: string;
  };
}

// Auth
export interface AuthUser {
  id: string;
  email: string;
  name: string;
  avatar_url?: string;
}

export interface AuthOrganization {
  id: string;
  name: string;
  slug: string;
  tier: string;
  timezone: string;
  pii_enabled: boolean;
}

export interface AuthMeResponse {
  user: AuthUser;
  organization: AuthOrganization;
}

export interface OAuthCallbackResponse {
  user: AuthUser;
  organization: { id: string; name: string; tier: string };
  is_new_user: boolean;
}

// Overview
export interface OverviewResponse {
  total_cost: number;
  total_requests: number;
  error_rate: number;
  avg_cost_per_request: number;
  avg_latency_ms: number;
  cost_change_percent: number | null;
  requests_change_percent: number | null;
  error_rate_change_percent: number | null;
  cost_per_req_change_percent: number | null;
  top_model?: string;
  top_feature?: string;
}

// Cost
export interface CostBreakdownItem {
  group: string;
  provider?: string;
  cost_usd: number;
  requests: number;
  tokens?: number;
  percent_of_total: number;
}

export interface TimeseriesPoint {
  timestamp: string;
  cost_usd: number;
  requests: number;
}

export interface TimeseriesSeries {
  group: string;
  points: TimeseriesPoint[];
}

export interface TimeseriesResponse {
  series: TimeseriesSeries[];
  granularity: "hour" | "day" | "week";
}

// Cost Compare
export interface CostComparePeriod {
  total_cost: number;
  requests: number;
}

export interface CostCompareChanges {
  cost_delta: number;
  cost_percent: number | null;
  request_percent: number | null;
}

export interface CostCompareBreakdownItem {
  group: string;
  current_cost: number;
  previous_cost: number;
  cost_change: number | null;
  requests: number;
}

export interface CostCompareResponse {
  current: CostComparePeriod;
  previous: CostComparePeriod;
  changes: CostCompareChanges;
  breakdown: CostCompareBreakdownItem[];
}

// Cost Filters
export interface CostFiltersResponse {
  models: string[];
  providers: string[];
  features: string[];
}

// API Keys
export interface APIKey {
  id: string;
  name: string;
  key_prefix: string;
  key?: string; // only present on creation
  project_id?: string;
  scopes: string[];
  last_used_at?: string;
  created_at: string;
  revoked_at?: string;
}

export interface CreateAPIKeyRequest {
  name: string;
  type: "live" | "test";
  project_id?: string;
}

// Settings
export interface OrgSettings {
  name: string;
  slug: string;
  timezone: string;
  pii_enabled: boolean;
  slack_webhook_url: string | null;
  tier: string;
}

export interface UpdateOrgSettingsRequest {
  name?: string;
  timezone?: string;
  pii_enabled?: boolean;
  slack_webhook_url?: string;
}

// Projects
export interface Project {
  id: string;
  name: string;
  slug: string;
  description: string;
  is_default: boolean;
  created_at: string;
}
