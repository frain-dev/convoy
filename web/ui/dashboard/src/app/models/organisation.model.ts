export interface ORGANIZATION_DATA {
	uid: string;
	members: ORGANIZATION_MEMBERS[];
	name: string;
}
export interface ORGANIZATION_MEMBERS {
	confirmed_at: string;
	email: string;
	id: string;
	profile: {
		firstname: string;
		lastname: string;
	};
}

export interface TEAM {
	role: {
		project: string[];
		type: string;
	};
	uid: string;
	status?: boolean;
	invitee_email?: string;
	user_metadata: {
		first_name: string;
		last_name: string;
		email: string;
	};
	created_at: string;
	deleted_at: string;
	organisation_id: string;
	updated_at: string;
	user_id: string;
}
