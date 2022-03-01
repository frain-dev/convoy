function AuthDetails() {
	const authDetails = localStorage.getItem('CONVOY_AUTH');
	if (authDetails) {
		const { token } = JSON.parse(authDetails);
		return { token, authState: true };
	} else {
		return { authState: false };
	}
}

// eslint-disable-next-line no-restricted-globals
const APIURL = `${location.port === '3000' ? 'http://localhost:5005' : location.origin}/ui`;

export { APIURL, AuthDetails };
