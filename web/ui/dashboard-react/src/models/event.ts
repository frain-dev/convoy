import type { ENDPOINT } from './endpoint.model';
import type { SOURCE } from './source';

export interface Event {
	uid: string;
	event_type: string;
	project_id: string;
	endpoints: null | Array<ENDPOINT>;
	headers: Record<string, string> | null;
	source_metadata: SOURCE;
	url_query_params: string;
	idempotency_key: string;
	is_duplicate_event: boolean;
	data: Record<string, unknown>;
	raw: string;
	status: string;
	acknowledged_at: string;
	created_at: string;
	updated_at: string;
	deleted_at: null | string;
	metadata?: {
		interval_seconds: number;
		next_send_time: string;
		num_trials: number;
		retry_limit: number;
		strategy: string;
		data?: Record<string, unknown>;
	};
	matched_endpoints?: number;
	endpoint_metadata?: Array<ENDPOINT>;
}

export interface EventDelivery {
	created_at: string;
	status: string;
	uid: string;
	updated_at: string;
	device_id?: string;
	description?: string;
	cli_metadata?: {
		event_type: string;
		host_name: string;
	};
	idempotency_key: string;
	metadata: {
		interval_seconds: number;
		next_send_time: string;
		num_trials: number;
		retry_limit: number;
		strategy: string;
		data: Record<string, unknown>;
	};
	source_metadata: SOURCE;
	endpoint_metadata: ENDPOINT;
	event_metadata?: Event;
	device_metadata?: {
		host_name: string;
	};
	endpoint_id: string;
}
