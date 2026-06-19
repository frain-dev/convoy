import {Injectable} from '@angular/core';
import {HttpService} from '../http/http.service';
import {HTTP_RESPONSE} from 'src/app/models/global.model';

@Injectable({
	providedIn: 'root'
})
export class LicensesService {
	readonly licensedOrgLabel = 'Pro';

	// Two license sets are kept side by side. The instance set is the deployment
	// license (self-hosted CONVOY_LICENSE_KEY or the cloud operator license); the
	// org set is the current org's plan entitlements. In cloud both carry real
	// limits, so org-context gating is the intersection (instance AND org): a
	// feature is granted only if both licenses allow it. The org plan is the
	// per-customer gate; the instance license is the platform capability and the
	// instance-wide limit. Self-hosted/OSS return the same set for both scopes,
	// so instance AND org collapses to the single license there.
	private readonly ORG_LICENSES_KEY = 'licenses';
	private readonly INSTANCE_LICENSES_KEY = 'instanceLicenses';

	constructor(private http: HttpService) {}

	private readLicenseData(storageKey: string): Record<string, any> | null {
		const raw = localStorage.getItem(storageKey);
		if (!raw) return null;
		try {
			return JSON.parse(raw) as Record<string, any>;
		} catch {
			return null;
		}
	}

	// Whether a single cache grants a boolean/limit feature. Fail closed: a
	// missing cache, missing feature, or denied value all read as not-allowed.
	private allowsInCache(storageKey: string, feature: string): boolean {
		const data = this.readLicenseData(storageKey);
		if (!data) return false;
		const v = data[feature];
		if (v && typeof v === 'object' && 'allowed' in v) return (v as { allowed: boolean }).allowed === true;
		if (typeof v === 'boolean') return v === true;
		return false;
	}

	// Limit block from a single cache, or null when the cache does not define it.
	private rawLimit(storageKey: string, limitKey: string): { current: number; limit: number; available: boolean; limit_reached: boolean } | null {
		const data = this.readLicenseData(storageKey);
		const l = data?.[limitKey];
		if (l && typeof l === 'object' && 'current' in l && 'limit' in l && 'available' in l && 'limit_reached' in l) {
			return {
				current: l.current || 0,
				limit: l.limit ?? 0,
				available: l.available === true,
				limit_reached: l.limit_reached === true
			};
		}
		return null;
	}

	// Treat -1 as unlimited so the smaller (more restrictive) limit wins.
	private normLimit(limit: number): number {
		return limit === -1 ? Number.POSITIVE_INFINITY : limit;
	}

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

	/**
	 * SSO/slug availability. The deployment must be SSO-capable (instance license)
	 * AND the org's plan must include it (not every cloud org gets SSO), so this is
	 * the instance AND org intersection like every other org-context feature.
	 */
	async hasEnterpriseSSO(): Promise<boolean> {
		return this.hasLicense('EnterpriseSSO');
	}

	/**
	 * Persist a single scope. instanceLevelOnly=true writes the deployment
	 * (instance) feature list; false writes the current org's plan features.
	 * Call with true from login/signup (public pages, no org context).
	 */
	async setLicenses(instanceLevelOnly = false) {
		try {
			const response = await this.getLicenses(undefined, instanceLevelOnly);
			const key = instanceLevelOnly ? this.INSTANCE_LICENSES_KEY : this.ORG_LICENSES_KEY;
			localStorage.setItem(key, JSON.stringify(response.data));
		} catch {}
	}

	/**
	 * Populate both the instance and org feature caches. Used by the
	 * authenticated dashboard bootstrap (and after a purchase) so instance
	 * platform features and org plan features are both fresh without one
	 * scope overwriting the other.
	 */
	async loadAllLicenses() {
		await Promise.all([this.setLicenses(true), this.setLicenses(false)]);
	}

	clearLicenses() {
		localStorage.removeItem(this.ORG_LICENSES_KEY);
		localStorage.removeItem(this.INSTANCE_LICENSES_KEY);
	}

	hasOrgLicense(org: { license_data?: string } | null): boolean {
		return !!(org?.license_data);
	}

	/**
	 * Org-context entitlement: granted only if both the instance and org licenses
	 * allow it (intersection). Used by all dashboard gates except the instance-only
	 * consumers below.
	 */
	hasLicense(license: string): boolean {
		return this.allowsInCache(this.INSTANCE_LICENSES_KEY, license) && this.allowsInCache(this.ORG_LICENSES_KEY, license);
	}

	/**
	 * Instance-only entitlement (deployment license, no org context). Use for
	 * public pages (signup/login) and instance-admin platform tools that no org
	 * plan grants (e.g. queue monitoring).
	 */
	hasInstanceLicense(license: string): boolean {
		return this.allowsInCache(this.INSTANCE_LICENSES_KEY, license);
	}

	isMultiUserMode(): boolean {
		return this.multiUserInCache(this.INSTANCE_LICENSES_KEY) && this.multiUserInCache(this.ORG_LICENSES_KEY);
	}

	private multiUserInCache(storageKey: string): boolean {
		const data = this.readLicenseData(storageKey);
		const userLimit = data?.['user_limit'];
		if (userLimit && typeof userLimit === 'object' && 'allowed' in userLimit && 'limit' in userLimit) {
			return userLimit.allowed === true && (userLimit.limit === -1 || userLimit.limit > 1);
		}
		return false;
	}

	// Both licenses must offer the limit for it to be available.
	isLimitAvailable(limitKey: string): boolean {
		const inst = this.rawLimit(this.INSTANCE_LICENSES_KEY, limitKey);
		const org = this.rawLimit(this.ORG_LICENSES_KEY, limitKey);
		const defined = [inst, org].filter((d): d is NonNullable<typeof d> => d !== null);
		if (defined.length === 0) return false;
		return defined.every(d => d.available === true);
	}

	// Reached if either license is at its limit.
	isLimitReached(limitKey: string): boolean {
		const inst = this.rawLimit(this.INSTANCE_LICENSES_KEY, limitKey);
		const org = this.rawLimit(this.ORG_LICENSES_KEY, limitKey);
		const defined = [inst, org].filter((d): d is NonNullable<typeof d> => d !== null);
		if (defined.length === 0) return false;
		return defined.some(d => d.limit_reached === true);
	}

	// Combined view: the more restrictive (smaller) limit, with that side's usage.
	getLimitInfo(limitKey: string): {current: number, limit: number, available: boolean, limit_reached: boolean} | null {
		const inst = this.rawLimit(this.INSTANCE_LICENSES_KEY, limitKey);
		const org = this.rawLimit(this.ORG_LICENSES_KEY, limitKey);
		if (!inst && !org) return null;
		if (!inst) return org;
		if (!org) return inst;

		const binding = this.normLimit(org.limit) <= this.normLimit(inst.limit) ? org : inst;
		return {
			current: binding.current,
			limit: binding.limit,
			available: inst.available && org.available,
			limit_reached: inst.limit_reached || org.limit_reached
		};
	}
}
