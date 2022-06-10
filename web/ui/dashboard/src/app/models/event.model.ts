export interface EVENT {
	created_at: string;
	provider_id: string;
	status?: string;
	uid: string;
	updated_at: string;
	app_id?: string;
	event_type: string;
	data: any;
	matched_endpoints: number;
	metadata?: {
		interval_seconds: number;
		next_send_time: Date;
		num_trials: number;
		retry_limit: number;
		strategy: string;
	};
	app_metadata: {
		group_id: string;
		support_email: string;
		name: string;
		uid: string;
	};
}

export interface EVENT_DELIVERY {
	created_at: string;
	status: string;
	uid: string;
	updated_at: string;
	app_id?: string;
	event_id?: string;
	metadata: {
		interval_seconds: number;
		next_send_time: string;
		num_trials: number;
		retry_limit: number;
		strategy: string;
	};
	endpoint: {
		secret: string;
		sent: boolean;
		status: string;
		target_url: string;
		uid: string;
	};
	app_metadata: {
		group_id: string;
		support_email: string;
		title: string;
		uid: string;
	};
	event_metadata: {
		event_type: string;
		uid: string;
		matched_endpoints: number;
	};
}

export interface EVENT_DELIVERY_ATTEMPT {
	ip_address: string;
	http_status: string;
	api_version: string;
	updated_at?: string;
	created_at: string;
	deleted_at?: number;
	response_data?: string;
	response_http_header: any;
	request_http_header: any;
	error?: string;
}
