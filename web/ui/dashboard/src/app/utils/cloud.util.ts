/**
 * Convoy Cloud serves every region, staging and dedicated tenant cluster under
 * `*.getconvoy.cloud`. Tenant hosts are provisioned at cluster-create time, so
 * an allowlist would go stale and the suffix is matched instead.
 *
 * This gate is what keeps cloud-only integrations off self-hosted installs: the
 * dashboard bundle is embedded in the self-hosted binary carrying the same
 * PostHog key, and no server-side cloud signal is available before login.
 */
export function isConvoyCloud(hostname: string = window.location.hostname): boolean {
	return hostname.endsWith('.getconvoy.cloud');
}
