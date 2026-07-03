// This file can be replaced during build by using the `fileReplacements` array.
// `ng build` replaces `environment.ts` with `environment.prod.ts`.
// The list of file replacements can be found in `angular.json`.

export const environment = {
	production: false,
	posthog: 'phc_lPJnjN5hrM8Dh7kgujIccs2xnGL2lmRv6UdOmOTCqEc',
	enterprise: false,
	// Dev backend origin. Absolute by default so a plain `ng serve` talks to a
	// local convoy on :5005 with no proxy. The `proxy` build config swaps this
	// for '' (same-origin) so `npm run start:proxy` routes through proxy.conf.js.
	apiOrigin: 'http://localhost:5005'
};

/*
 * For easier debugging in development mode, you can import the following file
 * to ignore zone related error stack frames such as `zone.run`, `zoneDelegate.invokeTask`.
 *
 * This import should be commented out in production mode because it will have a negative impact
 * on performance if an error is thrown.
 */
// import 'zone.js/plugins/zone-error';  // Included with Angular CLI.
