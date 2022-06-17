export interface TEAMS {
	role: {
		groups: string[];
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
}
