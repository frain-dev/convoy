export interface ENDPOINT {
  uid: string;
  title: string;
  advanced_signatures: boolean;
  failure_rate: number;
  authentication: {
    api_key: { header_value: string; header_name: string };
  };
  created_at: string;
  owner_id?: string;
  description: string;
  events?: unknown;
  status: 'paused' | 'active' | 'inactive';
  secrets?: SECRET[];
  name?: string;
  url: string;
  target_url: string;
  updated_at: string;
  rate_limit: number;
  rate_limit_duration: string;
  http_timeout?: string;
  support_email: string;
  slack_webhook_url?: string;
}

export interface SECRET {
  created_at: string;
  expires_at: string;
  uid: string;
  updated_at: string;
  value: string;
}

export interface PORTAL_LINK {
  uid: string;
  group_id: string;
  endpoint_count: number;
  endpoint: string[];
  endpoints_metadata: ENDPOINT[];
  can_manage_endpoint: boolean;
  name: string;
  owner_id: string;
  url: string;
  created_at: string;
  updated_at: string;
}

export interface API_KEY {
  created_at: Date;
  expires_at: Date;
  key_type: string;
  name: string;
  role: { type: string; group: string; endpoint: string };
  uid: string;
  updated_at: Date;
}

export interface EndpointFormValues {
  name: string;
  url: string;
  support_email: string;
  slack_webhook_url: string;
  secret: string | null;
  http_timeout: number | null;
  description: string | null;
  owner_id: string | null;
  rate_limit: number | null;
  rate_limit_duration: number | null;
  authentication?: {
    type: string;
    api_key: {
      header_name: string;
      header_value: string;
    }
  };
  advanced_signatures: boolean | null;
} 
