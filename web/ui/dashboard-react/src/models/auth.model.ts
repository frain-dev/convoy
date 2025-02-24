export interface CachedAuth {
	uid: string;
	first_name: string;
	last_name: string;
	email: string;
	email_verified: boolean;
	created_at: Date;
	updated_at: Date;
	deleted_at: Date | null;
	reset_password_expires_at: Date;
	auth_type: string;
	token: {
		access_token: string;
		refresh_token: string;
	};
}

export type CachedAuthTokens = CachedAuth['token'];
