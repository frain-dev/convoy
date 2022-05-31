export interface APP {
	endpoints: {
		uid: string;
		created_at: Date;
		description: string;
		events: any;
		status: string;
		secret: string;
		target_url: string;
		updated_at: Date;
	}[];
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
