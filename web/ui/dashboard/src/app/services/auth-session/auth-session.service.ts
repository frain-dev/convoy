import { Injectable } from '@angular/core';

@Injectable({
	providedIn: 'root'
})
export class AuthSessionService {
	clearLocalSession(userId?: string | null): void {
		localStorage.removeItem('CONVOY_AUTH');
		localStorage.removeItem('CONVOY_AUTH_TOKENS');
		localStorage.removeItem('CONVOY_LAST_USER_ID');
		localStorage.removeItem('CONVOY_PORTAL_LINK_AUTH_TOKEN');
		localStorage.removeItem('GOOGLE_OAUTH_ID_TOKEN');
		localStorage.removeItem('GOOGLE_OAUTH_USER_INFO');
		localStorage.removeItem('AUTH_TYPE');
		localStorage.removeItem('CONVOY_LAST_USER_ROLE');
		if (userId) {
			localStorage.removeItem(`CONVOY_LAST_USER_ROLE_${userId}`);
		}
	}
}
