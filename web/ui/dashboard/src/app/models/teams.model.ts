export interface TEAMS {
	firstname: string;
	role: {
		groups: string[];
		type: string;
	};
	uid: string;
	lastname: string;
	status: boolean;
}
