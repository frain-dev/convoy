export interface PROJECT {
	uid: string;
	name: string;
	logo_url: string;
	config: {
		ratelimit: {
			count: number;
			duration: string;
		};
		disable_endpoint: boolean;
		strategy: {
			type: string;
			retry_count: number;
			duration: number;
		};
		signature: {
			header: string;
			versions: VERSIONS[];
		};
		meta_event: {
			event_type: string[];
			is_enabled: boolean;
			secret: string;
			type: string;
			url: string;
		};
		retention_policy: {
			policy: string;
			search_policy: string;
		};
		DisableEndpoint: boolean;
		replay_attacks: boolean;
	};
	statistics?: {
		messages_sent: number;
		total_endpoints: number;
	};
	created_at: Date;
	updated_at: Date;
	type: 'incoming' | 'outgoing';
	selected?: boolean;
	organisation_id: string;
	rate_limit_duration: string;
	rate_limit: string;
}

export interface VERSIONS {
	created_at: Date;
	encoding: string;
	hash: string;
	uid: string;
}

export interface META_EVENT {
	attempt: {
		request_http_header: object;
		response_http_header: object;
	};
	created_at: string;
	deleted_at: string;
	event_type: string;
	metadata: {
		data: object;
		interval_seconds: number;
		next_send_time: string;
		num_trials: number;
		raw: string;
		retry_limit: number;
		strategy: string;
	};
	project_id: string;
	status: string;
	uid: string;
	updated_at: string;
}
