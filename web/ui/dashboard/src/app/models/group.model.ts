export interface GROUP {
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

export interface SOURCE {
	created_at: Date;
	deleted_at: number;
	group_id: string;
	is_disabled: boolean;
	mask_id: string;
	name: string;
	type: string;
	uid: string;
	updated_at: number;
	url: string;
	provider: string;
	provider_config?: { twitter: { crc_verified_at: Date } };
	verifier: {
		api_key: {
			header_name: string;
			header_value: string;
		};
		basic_auth: {
			password: string;
			username: string;
		};
		hmac: {
			encoding: string;
			hash: string;
			header: string;
			secret: string;
		};
		type: string;
	};
	pub_sub: {
		google: {
			service_account: string;
			subscription_id: string;
			project_id: string;
		};
		sqs: {
			access_key_id: string;
			default_region: string;
			queue_name: string;
			secret_key: string;
		};
		type: string;
		workers: number;
	};
}

export interface VERSIONS {
	created_at: Date;
	encoding: string;
	hash: string;
	uid: string;
}
