import { consumeInviteRedirect, rememberInviteRedirect, sessionHasAccessToken } from './invite-redirect';

describe('invite-redirect', () => {
	beforeEach(() => {
		localStorage.clear();
	});

	afterEach(() => {
		localStorage.clear();
	});

	describe('sessionHasAccessToken', () => {
		it('is false when CONVOY_AUTH_TOKENS is missing', () => {
			localStorage.setItem('CONVOY_AUTH', JSON.stringify({ email: 'invitee@test.com' }));
			expect(sessionHasAccessToken()).toBe(false);
		});

		it('is false when access_token is empty', () => {
			localStorage.setItem('CONVOY_AUTH_TOKENS', JSON.stringify({ access_token: '', refresh_token: 'r' }));
			expect(sessionHasAccessToken()).toBe(false);
		});

		it('is true when access_token is a non-empty string', () => {
			localStorage.setItem('CONVOY_AUTH_TOKENS', JSON.stringify({ access_token: 'jwt', refresh_token: 'r' }));
			expect(sessionHasAccessToken()).toBe(true);
		});
	});

	describe('rememberInviteRedirect', () => {
		it('stores a relative accept-invite path', () => {
			expect(rememberInviteRedirect('/accept-invite?invite-token=abcDEF12')).toBe('/accept-invite?invite-token=abcDEF12');
			expect(consumeInviteRedirect()).toBe('/accept-invite?invite-token=abcDEF12');
		});

		it('rejects an absolute URL', () => {
			expect(rememberInviteRedirect('https://evil.example/accept-invite?invite-token=abcDEF12')).toBeNull();
		});
	});
});
