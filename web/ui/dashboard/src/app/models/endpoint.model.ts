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
	authentication: {
		api_key: { header_value: string; header_name: string };
	};
	created_at: string;
    owner_id?:string;
	description: string;
	events?: any;
	status?: string;
	secrets?: SECRET[];
	name?: string;
	url: string;
	updated_at: string;
	rate_limit: number;
	rate_limit_duration: string;
	http_timeout?: string;
	support_email: string;
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
