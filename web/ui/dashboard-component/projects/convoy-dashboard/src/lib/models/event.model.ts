export interface EVENT {
	created_at: Date;
	provider_id: string;
	status: string;
	uid: string;
	updated_at: Date;
	app_id: string;
	event_type: string;
	metadata: {
		interval_seconds: number;
		next_send_time: Date;
		num_trials: number;
		retry_limit: number;
		strategy: string;
	};
	app_metadata: {
		group_id: string;
		support_email: string;
		title: string;
		uid: string;
	};
}

export interface EVENT_DELIVERY {
	created_at: Date;
	status: string;
	uid: string;
	updated_at: Date;
	app_id: string;
	event_id: string;
	metadata: {
		interval_seconds: number;
		next_send_time: Date;
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
		name: string;
		uid: string;
	};
}

export interface EVENT_DELIVERY_ATTEMPT {
	ip_address: string;
	http_status: string;
	api_version: string;
	updated_at: number;
	deleted_at: number;
	response_data: string;
	response_http_header: string;
	request_http_header: string;
	error: string;
}

export interface EVENT_TYPE {
	type: 'events' | 'eventDeliveries';
}

export interface TIME_DATA {
	startDate: string;
	endDate: string;
}

export interface EVENT_TIME extends EVENT_TYPE, TIME_DATA {}
