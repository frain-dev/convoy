export interface ORGANIZATION_MEMBERS {
	confirmed_at: string;
	email: string;
	id: string;
	profile: {
		firstname: string;
		lastname: string;
	};
}

export interface ORGANIZATION_DATA {
	uid: string;
	members: ORGANIZATION_MEMBERS[];
	name: string;
}
