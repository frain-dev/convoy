const STORAGE_KEY = 'CONVOY_INVITE_REDIRECT';

export function inviteAcceptPath(token: string): string {
	return `/accept-invite?invite-token=${encodeURIComponent(token)}`;
}

// Same store HttpService.authDetails reads. Email in CONVOY_AUTH is not a session.
export function sessionHasAccessToken(): boolean {
	try {
		const raw = localStorage.getItem('CONVOY_AUTH_TOKENS');
		if (!raw || raw === 'undefined') return false;
		const tokens = JSON.parse(raw);
		return typeof tokens?.access_token === 'string' && tokens.access_token.trim().length > 0;
	} catch {
		return false;
	}
}

export function rememberInviteRedirect(raw: string | null | undefined): string | null {
	const safe = sanitizeInviteRedirect(raw || '');
	if (!safe) return null;
	try {
		localStorage.setItem(STORAGE_KEY, safe);
	} catch {
		// ignore quota / private-mode failures; the query param still works
	}
	return safe;
}

export function consumeInviteRedirect(queryRedirect?: string | null): string | null {
	let raw = typeof queryRedirect === 'string' ? queryRedirect : '';
	if (!raw) {
		try {
			raw = localStorage.getItem(STORAGE_KEY) || '';
		} catch {
			raw = '';
		}
	}
	const safe = sanitizeInviteRedirect(raw);
	if (safe) {
		try {
			localStorage.removeItem(STORAGE_KEY);
		} catch {
			// ignore
		}
	}
	return safe;
}

export function sanitizeInviteRedirect(raw: string): string | null {
	if (typeof raw !== 'string' || raw.length === 0) return null;
	let url: URL;
	try {
		url = new URL(raw, 'http://local.invalid');
	} catch {
		return null;
	}
	if (url.origin !== 'http://local.invalid') return null;
	if (url.pathname !== '/accept-invite') return null;
	const token = url.searchParams.get('invite-token');
	if (typeof token !== 'string' || !/^[A-Za-z0-9_-]{8,128}$/.test(token)) return null;
	return inviteAcceptPath(token);
}
