// Dev-only proxy for `ng serve`, used by `npm run start:proxy` (the `proxy` build
// config makes the dashboard call its backend same-origin; see
// src/app/services/api-origin.ts). This forwards the backend path prefixes to a
// local convoy server. Set CONVOY_DEV_BACKEND to target a specific instance/port;
// it defaults to the standard local server on :5005.
const target = process.env.CONVOY_DEV_BACKEND || 'http://localhost:5005';

module.exports = ['/ui', '/portal-api', '/queue'].reduce((config, prefix) => {
	config[prefix] = { target, secure: false, changeOrigin: true, ws: true };
	return config;
}, {});
