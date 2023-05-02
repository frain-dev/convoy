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
