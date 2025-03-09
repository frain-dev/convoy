import { request } from './http.service';

export type License = Record<string, { allowed: boolean }>;

export async function getLicenses(
	deps: { httpReq: typeof request } = { httpReq: request },
) {
	const res = await deps.httpReq<Array<License>>({
		url: '/license/features',
		method: 'get',
	});

	let allowedLicenses: Array<LicenseKey> = [];
	if (res) {
		allowedLicenses = Object.entries(res.data).reduce<Array<LicenseKey>>(
			(acc, [key, { allowed }]) => {
				if (allowed) return acc.concat(key as LicenseKey);
				return acc as Array<LicenseKey>;
			},
			[],
		);
	}
	return allowedLicenses;
}

const LICENSES = [
	'ADVANCED_ENDPOINT_MANAGEMENT',
	'ADVANCED_MESSAGE_BROKER',
	'ADVANCED_SUBSCRIPTIONS',
	'ADVANCED_WEBHOOK_ARCHIVING',
	'ADVANCED_WEBHOOK_FILTERING',
	'AGENT_EXECUTION_MODE',
	'ASYNQ_MONITORING',
	'AUDIT_LOGS',
	'CIRCUIT_BREAKING',
	'CONSUMER_POOL_TUNING',
	'CREATE_ORG',
	'CREATE_PROJECT',
	'CREATE_USER',
	'CREDENTIAL_ENCRYPTION',
	'DATADOG_TRACING',
	'ENTERPRISE_SSO',
	'EVENT_CATALOGUE',
	'EXPORT_PROMETHEUS_METRICS',
	'INGEST_RATE',
	'IP_RULES',
	'MULTI_PLAYER_MODE',
	'PORTAL_LINKS',
	'SYNCHRONOUS_WEBHOOKS',
	'USE_FORWARD_PROXY',
	'WEBHOOK_ANALYTICS',
	'WEBHOOK_TRANSFORMATIONS',
] as const;

export type LicenseKey = (typeof LICENSES)[number];
