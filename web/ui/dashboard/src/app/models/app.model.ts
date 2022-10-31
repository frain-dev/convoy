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
	advanced_signatures: boolean;
	authentication: any;
	created_at: string;
	description: string;
	events?: any;
	status?: string;
	secrets?: SECRET[];
	name?: string;
	target_url: string;
	updated_at: string;
	rate_limit?: number;
	rate_limit_duration?: string;
	http_timeout?: string;
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

export interface API_KEY {
	created_at: Date;
	expires_at: Date;
	key_type: string;
	name: string;
	role: { type: string; group: string; app: string };
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
