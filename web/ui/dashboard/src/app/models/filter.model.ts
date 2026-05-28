export interface FILTER {
	uid: string;
	subscription_id: string;
	event_type: string;
	enabled_at?: string | null;
	headers: any;
	body: any;
	query?: any;
	path?: any;
	is_new?: boolean;
	is_modified?: boolean;
	raw_headers?: any;
	raw_body?: any;
	raw_query?: any;
	raw_path?: any;
	created_at?: string;
	updated_at?: string;
}

export interface FILTER_CREATE_REQUEST {
	subscription_id: string;
	event_type: string;
	enabled_at?: string | null;
	headers?: any;
	body?: any;
	query?: any;
	path?: any;
	raw_headers?: any;
	raw_body?: any;
	raw_query?: any;
	raw_path?: any;
}

export interface FILTER_TEST_REQUEST {
	subscription_id: string;
	event_type: string;
	sample_payload: any;
}

export interface FILTER_TEST_RESPONSE {
	data: boolean;
}
