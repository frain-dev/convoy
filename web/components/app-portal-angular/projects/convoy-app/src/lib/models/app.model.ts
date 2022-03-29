export interface APP {
	endpoints: {
		uid: string;
		created_at: Date;
		description: string;
		status: string;
		target_url: string;
		updated_at: Date;
		events: string[];
	}[];
	events: number;
	group_id: string;
	is_disabled: boolean;
	name: string;
	secret: string;
	support_email: string;
	created_at: Date;
	uid: string;
	updated_at: Date;
}
