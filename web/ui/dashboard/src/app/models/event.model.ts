import { DEVICE } from './device.model';
import { APP, ENDPOINT } from './endpoint.model';
import { SOURCE } from './source.model';

export interface EVENT {
	created_at: Date;
	provider_id: string;
	status?: string;
	uid: string;
	updated_at: string;
	app_id?: string;
	idempotency_key: string;
	is_duplicate_event: boolean;
	event_type: string;
	data: any;
	matched_endpoints: number;
	endpoint_metadata: ENDPOINT[];
	metadata?: {
		interval_seconds: number;
		next_send_time: Date;
		num_trials: number;
		retry_limit: number;
		strategy: string;
	};
	app_metadata: APP;
	source_metadata: SOURCE;
}

export interface EVENT_DELIVERY {
	created_at: string;
	status: string;
	uid: string;
	updated_at: string;
	device_id: string;
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
		data: any;
	};
	source_metadata: SOURCE;
	endpoint_metadata: ENDPOINT;
	app_metadata: APP;
	event_metadata: EVENT;
	device_metadata: DEVICE;
	endpoint_id: string;
}

export interface EVENT_DELIVERY_ATTEMPT {
	ip_address: string;
	http_status: string;
	api_version: string;
	updated_at: string;
	created_at: string;
	deleted_at?: number;
	status?: boolean;
	response_data?: string;
	response_http_header: any;
	request_http_header: any;
	uid: string;
	error?: string;
}

export interface FILTER_QUERY_PARAM {
	startDate?: string;
	endDate?: string;
	eventId?: string;
	endpointId?: string;
	idempotencyKey?: string;
	status?: string;
	sourceId?: string;
	next_page_cursor?: string;
	prev_page_cursor?: string;
	direction?: 'next' | 'prev';
	showLoader?: boolean;
	query?: string;
	name?: string;
	sort?: string;
	eventType?: string;
}
