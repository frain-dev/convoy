export interface FILTER {
	uid: string;
	subscription_id: string;
	event_type: string;
	headers: any;
	body: any;
	is_new?: boolean;
	raw_headers?: any;
	raw_body?: any;
	created_at?: string;
	updated_at?: string;
}

export interface FILTER_CREATE_REQUEST {
	subscription_id: string;
	event_type: string;
	headers?: any;
	body?: any;
	raw_headers?: any;
	raw_body?: any;
}

export interface FILTER_TEST_REQUEST {
	subscription_id: string;
	event_type: string;
	sample_payload: any;
}

export interface FILTER_TEST_RESPONSE {
	data: boolean;
}
