export interface OrganisationMember {
	confirmed_at: string;
	email: string;
	id: string;
	profile: {
		firstname: string;
		lastname: string;
	};
}

export interface Organisation {
	uid: string;
	OwnerID: string;
	name: string;
	custom_domain: null | string;
	assigned_domain: null | String;
	members: OrganisationMember[] | null;
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
