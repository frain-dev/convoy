// Dev + proxy build config. Same as environment.ts but apiOrigin is empty so the
// dashboard calls its backend same-origin and `proxy.conf.js` forwards the backend
// path prefixes to CONVOY_DEV_BACKEND (default http://localhost:5005). Selected by
// `npm run start:proxy` (ng serve -c proxy). Lets a contributor point the UI at any
// local convoy instance/port without editing source.
export const environment = {
	production: false,
	posthog: 'phc_lPJnjN5hrM8Dh7kgujIccs2xnGL2lmRv6UdOmOTCqEc',
	enterprise: false,
	apiOrigin: ''
};
