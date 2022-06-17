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
		Strategy: {
			type: string;
			default: {
				intervalSeconds: number;
				retryLimit: number;
			};
		};
		Signature: {
			header: string;
			hash: string;
		};
		DisableEndpoint: boolean;
		replay_attacks: boolean;
	};
	statistics: {
		messages_sent: number;
		total_apps: number;
	};
	created_at: Date;
	updated_at: Date;
	type: 'incoming' | 'outgoing';
	selected?: boolean;
}

export interface SOURCE {
	created_at: number;
	deleted_at: number;
	group_id: string;
	is_disabled: boolean;
	mask_id: string;
	name: string;
	type: string;
	uid: string;
	updated_at: number;
	url: string;
	verifier: {
		api_key: {
			header: string;
			key: string;
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
}
