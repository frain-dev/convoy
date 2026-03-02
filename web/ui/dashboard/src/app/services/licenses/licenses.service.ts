import {Injectable} from '@angular/core';
import {HttpService} from '../http/http.service';
import {HTTP_RESPONSE} from 'src/app/models/global.model';

@Injectable({
	providedIn: 'root'
})
export class LicensesService {
	constructor(private http: HttpService) {}

	getLicenses(orgId?: string, instanceLevelOnly = false): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			const query: Record<string, string> = {};
			if (!instanceLevelOnly) {
				const fromStorage = this.http.getOrganisation();
				const org = orgId ?? fromStorage?.uid;
				if (org) query['orgID'] = org;
			}
			const queryUndefined = Object.keys(query).length === 0 ? undefined : query;
			try {
				const response = await this.http.request({
					url: `/license/features`,
					method: 'get',
					query: queryUndefined
				});
				return resolve(response);
			} catch (error) {
				return reject(error);
			}
		});
	}

	/** Call from login/signup (public pages) so the request uses instance-level only (no orgID). */
	async setLicenses(instanceLevelOnly = false) {
		try {
			const response = await this.getLicenses(undefined, instanceLevelOnly);
			localStorage.setItem('licenses', JSON.stringify(response.data));
		} catch {}
	}

	hasOrgLicense(org: { license_data?: string } | null): boolean {
		return !!(org?.license_data);
	}

	hasLicense(license: string): boolean {
		const savedLicenses = localStorage.getItem('licenses');
		if (!savedLicenses) {
			return false;
		}

		try {
			const licenseData = JSON.parse(savedLicenses);

			// Check if it's a limit (object with allowed field)
			if (licenseData[license] && typeof licenseData[license] === 'object' && 'allowed' in licenseData[license]) {
				return licenseData[license].allowed === true;
			}

			// Check if it's a boolean feature
			if (typeof licenseData[license] === 'boolean') {
				return licenseData[license] === true;
			}

			return false;
		} catch {
			return false;
		}
	}

	isMultiUserMode(): boolean {
		const savedLicenses = localStorage.getItem('licenses');
		if (!savedLicenses) {
			return false;
		}

		try {
			const licenseData = JSON.parse(savedLicenses);
			const userLimit = licenseData['user_limit'];
			if (userLimit && typeof userLimit === 'object' && 'allowed' in userLimit && 'limit' in userLimit) {
				return userLimit.allowed === true && userLimit.limit > 1;
			}
			return false;
		} catch {
			return false;
		}
	}

	isLimitAvailable(limitKey: string): boolean {
		const savedLicenses = localStorage.getItem('licenses');
		if (!savedLicenses) {
			return false;
		}

		try {
			const licenseData = JSON.parse(savedLicenses);
			const limit = licenseData[limitKey];
			if (limit && typeof limit === 'object' && 'available' in limit) {
				return limit.available === true;
			}
			return false;
		} catch {
			return false;
		}
	}

	isLimitReached(limitKey: string): boolean {
		const savedLicenses = localStorage.getItem('licenses');
		if (!savedLicenses) {
			return false;
		}

		try {
			const licenseData = JSON.parse(savedLicenses);
			const limit = licenseData[limitKey];
			if (limit && typeof limit === 'object' && 'limit_reached' in limit) {
				return limit.limit_reached === true;
			}
			return false;
		} catch {
			return false;
		}
	}

	getLimitInfo(limitKey: string): {current: number, limit: number, available: boolean, limit_reached: boolean} | null {
		const savedLicenses = localStorage.getItem('licenses');
		if (!savedLicenses) {
			return null;
		}

		try {
			const licenseData = JSON.parse(savedLicenses);
			const limit = licenseData[limitKey];
			if (limit && typeof limit === 'object' && 'current' in limit && 'limit' in limit && 'available' in limit && 'limit_reached' in limit) {
				return {
					current: limit.current || 0,
					limit: limit.limit || 0,
					available: limit.available === true,
					limit_reached: limit.limit_reached === true
				};
			}
			return null;
		} catch {
			return null;
		}
	}
}
