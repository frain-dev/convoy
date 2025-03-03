export interface Project {
	uid: string;
	name: string;
	logo_url: string;
	organisation_id: string;
	type: 'incoming' | 'outgoing';
	retained_events: number;
	created_at: Date;
	updated_at: Date;
	deleted_at: Date | null;
	config: {
		disable_endpoint: boolean;
		retention_policy_enabled: boolean;
		DisableEndpoint: boolean;
		replay_attacks: boolean;
		search_policy: string;
		ratelimit: {
			count: number;
			duration: number;
		};
		strategy: {
			type: string;
			retry_count: number;
			duration: number;
		};
		signature: {
			header: string;
			versions: Version[];
		};
		ssl: {
			enforce_secure_endpoints: boolean;
		};
		meta_event: {
			event_type: string[] | null;
			is_enabled: boolean;
			secret: string;
			type: string;
			url: string;
		};
		// retention_policy: {
		// 	policy: string;
		// 	search_policy: string;
		// };
	};
	statistics: {
		events_exist: boolean;
		subscriptions_exist: boolean;
		endpoints_exist: boolean;
	} | null;
}

export interface Version {
	created_at: Date;
	encoding: string;
	hash: string;
	uid: string;
}

export interface MetaEvent {
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
