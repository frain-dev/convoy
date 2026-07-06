export const environment = {
	production: true,
	posthog: 'phc_lPJnjN5hrM8Dh7kgujIccs2xnGL2lmRv6UdOmOTCqEc',
	enterprise: false,
	// Unused in production (apiOrigin() returns location.origin); kept for a
	// consistent environment shape across build configs.
	apiOrigin: ''
};
