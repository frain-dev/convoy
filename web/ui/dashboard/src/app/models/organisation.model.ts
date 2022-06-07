export interface ORGANIZATION_DATA {
	id: string;
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
