export interface APP {
	endpoints: ENDPOINT[];
	events: number;
	group_id: string;
	name: string;
	secret: string;
	support_email: string;
	is_disabled: boolean;
	created_at: Date;
	uid: string;
	updated_at: Date;
}
export interface ENDPOINT {
	uid: string;
	title: string;
	advanced_signatures: boolean;
	failure_rate: number;
	authentication: {
		type?: 'api_key' | 'oauth2';
		api_key?: { header_value: string; header_name: string };
		oauth2?: {
			url: string;
			client_id: string;
			authentication_type: 'shared_secret' | 'client_assertion';
			client_secret?: string;
			grant_type?: string;
			scope?: string;
			signing_key?: {
				kty: string;
				// EC (Elliptic Curve) key fields
				crv?: string;
				x?: string;
				y?: string;
				d: string;
				// RSA key fields
				n?: string;
				e?: string;
				p?: string;
				q?: string;
				dp?: string;
				dq?: string;
				qi?: string;
				// Common fields
				kid: string;
			};
			signing_algorithm?: string;
			issuer?: string;
			subject?: string;
			field_mapping?: {
				access_token?: string;
				token_type?: string;
				expires_in?: string;
			};
			expiry_time_unit?: 'seconds' | 'milliseconds' | 'minutes' | 'hours';
		};
	};
	created_at: string;
    owner_id?:string;
	description: string;
	events?: any;
	status: string;
	secrets?: SECRET[];
	name?: string;
	url: string;
	target_url: string;
	updated_at: string;
	rate_limit: number;
	rate_limit_duration: string;
	http_timeout?: string;
	support_email: string;
	content_type?: string;
	mtls_client_cert?: {
		client_cert?: string;
		client_key?: string;
	};
}

export interface DEVICE {
	uid: string;
	group_id: string;
	app_id: string;
	host_name: string;
	status: string;
	last_seen_at: Date;
	created_at: Date;
	updated_at: Date;
}

export interface PORTAL_LINK {
	uid: string;
	group_id: string;
	endpoint_count: number;
	endpoint: string[];
	endpoints_metadata: ENDPOINT[];
	can_manage_endpoint: boolean;
	name: string;
	owner_id: string;
	url: string;
	created_at: string;
	updated_at: string;
}

export interface API_KEY {
	created_at: Date;
	expires_at: Date;
	key_type: string;
	name: string;
	role: { type: string; group: string; endpoint: string };
	uid: string;
	updated_at: Date;
}

export interface SECRET {
	created_at: string;
	expires_at: string;
	uid: string;
	updated_at: string;
	value: string;
}
