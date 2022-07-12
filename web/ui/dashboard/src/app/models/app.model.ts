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
	created_at: Date;
	description: string;
	events: any;
	status: string;
	secret: string;
	name?: string;
	target_url: string;
	updated_at: Date;
	rate_limit?: number;
	rate_limit_duration?: string;
	http_timeout?: string;
}
