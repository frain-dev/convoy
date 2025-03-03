import { request } from './http.service';
import { CONVOY_LICENSES_KEY } from '@/lib/constants';

type License = Record<string, { allowed: boolean }>;

export async function getLicenses(
	deps: { httpReq: typeof request } = { httpReq: request },
) {
	const res = await deps.httpReq<Array<License>>({
		url: '/license/features',
		method: 'get',
	});

	return res;
}

type SetLicensesDeps = {
	httpReq: typeof request;
	getLicenses: typeof getLicenses;
};

export async function setLicenses(
	deps: SetLicensesDeps = { httpReq: request, getLicenses },
) {
	const res = await deps.getLicenses();

	if (res) {
		const allowedLicenses = Object.entries(res.data).reduce<Array<string>>(
			(acc, [key, { allowed }]) => {
				if (allowed) return acc.concat(key);
				return acc;
			},
			[],
		);

		localStorage.setItem(CONVOY_LICENSES_KEY, JSON.stringify(allowedLicenses));
	}
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

type LicenseKey = (typeof LICENSES)[number];

export function hasLicense(license: LicenseKey): boolean {
	const savedLicenses = localStorage.getItem(CONVOY_LICENSES_KEY);

	if (savedLicenses) {
		const licenses: Array<string> = JSON.parse(savedLicenses);
		const userHasLicense = licenses.includes(license);
		return userHasLicense;
	}

	return false;
}
