type Statistics = {
	events_exist: boolean;
	subscriptions_exist: boolean;
	endpoints_exist: boolean;
} | null;

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
		replay_attacks: boolean;
		search_policy: string;
		ratelimit: {
			count: number;
			duration: number;
		};
		strategy: {
			type: 'linear' | 'exponential';
			retry_count: number;
			duration: number;
		};
		signature: {
			header: string;
			versions: Version[];
		};
		ssl: Ssl;
		meta_event: {
			// FIXME I think `event_type` should be `event_types` or `types`
			event_type:  Array<typeof EventTypes[number]> | null;
			is_enabled: boolean;
			secret: string;
			type: string;
			url: string;
			// pub_sub: null; // It's in the backend but not in the frontend model
		};
		multiple_endpoint_subscriptions: boolean;
		// retention_policy: {
		// 	policy: string;
		// 	search_policy: string;
		// };
	};
	statistics: Statistics;
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

export interface CreateProjectResponse {
	api_key: ApiKey;
	project: {
		uid: string;
		name: string;
		logo_url: string;
		organisation_id: string;
		type: string;
		config: Config;
		statistics: Statistics;
		retained_events: number;
		created_at: string;
		updated_at: string;
		deleted_at: null | string;
	};
}

interface ApiKey {
	name: string;
	role: Role;
	key_type: string;
	expires_at: null | string;
	/**
	 * Shown only once on the UI
	 */
	key: string;
	uid: string;
	created_at: string;
}

interface Role {
	type: 'admin' | 'super_admin' | 'member';
	project: string;
}

interface Config {
	max_payload_read_size: number;
	replay_attacks_prevention_enabled: boolean;
	add_event_id_trace_headers: boolean;
	disable_endpoint: boolean;
	multiple_endpoint_subscriptions: boolean;
	search_policy: string;
	ssl: Ssl;
	ratelimit: Ratelimit;
	strategy: Strategy;
	signature: Signature;
	meta_event: MetaEvent;
}

interface Ssl {
	enforce_secure_endpoints: boolean;
}

interface Ratelimit {
	count: number;
	duration: number;
}

interface Strategy {
	type: string;
	duration: number;
	retry_count: number;
}

interface Signature {
	header: string;
	versions: Version[];
}

export interface Version {
	uid: string;
	hash: string;
	encoding: string;
	created_at: string;
}

export const EventTypes = [
	'endpoint.created',
	'endpoint.deleted',
	'endpoint.updated',
	'eventdelivery.success',
	'eventdelivery.failed',
	'project.updated',
] as const; 

export type EventType = typeof EventTypes[number]
