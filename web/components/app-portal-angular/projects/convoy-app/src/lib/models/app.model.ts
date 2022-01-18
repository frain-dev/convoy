export interface APP {
	endpoints: {
		uid: string;
		created_at: Date;
		description: string;
		status: string;
		target_url: string;
		updated_at: Date;
	}[];
	events: 2;
	group_id: string;
	name: string;
	secret: string;
	support_email: string;
	created_at: Date;
	uid: string;
	updated_at: Date;
}
