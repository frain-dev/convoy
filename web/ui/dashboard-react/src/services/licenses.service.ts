import { request } from './http.service';

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
		localStorage.setItem('licenses', JSON.stringify(allowedLicenses));
	}
}

type LicenseKey = 'CREATE_USER' | 'CREATE_PROJECT' | 'CREATE_ORG';

export function hasLicense(license: LicenseKey): boolean {
	const savedLicenses = localStorage.getItem('licenses');

	if (savedLicenses) {
		const licenses: Array<string> = JSON.parse(savedLicenses);
		const userHasLicense = licenses.includes(license);
		return userHasLicense;
	}

	return false;
}
