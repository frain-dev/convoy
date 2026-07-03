import { environment } from 'src/environments/environment';

// apiOrigin returns the base origin for backend calls. Kept in one place so the
// dev/prod backend target is never duplicated across services.
// - default dev (`ng serve`): environment.apiOrigin is an absolute URL
//   (http://localhost:5005) so the dashboard talks to a local convoy directly.
// - `proxy` dev config (`npm run start:proxy`): environment.apiOrigin is empty, so
//   we fall back to same-origin and proxy.conf.js forwards the backend prefixes to
//   CONVOY_DEV_BACKEND.
// - production: served by convoy itself, so same-origin is correct.
export function apiOrigin(): string {
	if (!environment.production && environment.apiOrigin) {
		return environment.apiOrigin;
	}
	return location.origin;
}
