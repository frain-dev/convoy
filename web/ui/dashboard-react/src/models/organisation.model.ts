export interface Member {
	uid: string;
	organisation_id: string;
	user_id: string;
	role: {
		type: 'super_user' | 'admin' | string;
		project: string;
	};
	user_metadata: {
		first_name: string;
		last_name: string;
		email: string;
	};
	created_at: string;
	updated_at: string;
	deleted_at: null;
}

export interface Organisation {
	uid: string;
	OwnerID: string;
	name: string;
	custom_domain: null | string;
	assigned_domain: null | String;
	/**
	 * Date string
	 */
	created_at: Date;
	/**
	 * Date string
	 */
	updated_at: Date;
	deleted_at: null | Date;
}
