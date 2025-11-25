import {Component, inject, OnInit} from '@angular/core';
import {SettingsService} from '../settings.service';
import {GeneralService} from 'src/app/services/general/general.service';
import {RbacService} from 'src/app/services/rbac/rbac.service';
import {LicensesService} from 'src/app/services/licenses/licenses.service';

interface EarlyAdopterFeature {
	key: string;
	name: string;
	description: string;
	enabled: boolean;
}

@Component({
	selector: 'early-adopter-features',
	templateUrl: './early-adopter-features.component.html',
	styleUrls: ['./early-adopter-features.component.scss']
})
export class EarlyAdopterFeaturesComponent implements OnInit {
	organisationId!: string;
	isLoadingFeatures = false;
	isUpdatingFeatures = false;
	earlyAdopterFeatures: EarlyAdopterFeature[] = [];
	private rbacService = inject(RbacService);
	canManage = false;

	constructor(
		private settingService: SettingsService,
		private generalService: GeneralService,
		private licenseService: LicensesService
	) {}

	async ngOnInit() {
		const org = localStorage.getItem('CONVOY_ORG');
		if (org) {
			const organisationDetails = JSON.parse(org);
			this.organisationId = organisationDetails.uid;
		}

		this.canManage = await this.rbacService.userCanAccess('Organisations|MANAGE');
		await this.getEarlyAdopterFeatures();
	}

	async getEarlyAdopterFeatures() {
		if (!this.organisationId) return;
		this.isLoadingFeatures = true;
		try {
			const response = await this.settingService.getEarlyAdopterFeatures({ org_id: this.organisationId });
			const allFeatures = response.data || [];

			// Filter features based on license availability
			this.earlyAdopterFeatures = allFeatures.filter((feature: EarlyAdopterFeature) => {
				return this.hasFeatureLicense(feature.key);
			});

			this.isLoadingFeatures = false;
		} catch (error) {
			console.error('Error fetching features:', error);
			this.isLoadingFeatures = false;
		}
	}

	private hasFeatureLicense(featureKey: string): boolean {
		const licenseMap: { [key: string]: string } = {
			'mtls': 'MUTUAL_TLS',
			'oauth-token-exchange': 'OAUTH2_ENDPOINT_AUTH'
		};

		const license = licenseMap[featureKey];
		if (!license) {
			return false;
		}

		return this.licenseService.hasLicense(license);
	}

	trackByFeatureKey(index: number, feature: EarlyAdopterFeature): string {
		return feature.key;
	}

	handleToggleChange(event: Event) {
		const input = event?.target as HTMLInputElement | null;

		if (!input) {
			console.error('Toggle event missing target', event);
			return;
		}

		const prefix = 'feature-';
		const featureKey = input.id?.startsWith(prefix) ? input.id.slice(prefix.length) : input.id;

		if (!featureKey) {
			console.error('Unable to determine feature key from toggle event', event);
			return;
		}

		this.toggleFeature(featureKey);
	}

	async toggleFeature(featureKey: string) {
		if (!this.organisationId || this.isUpdatingFeatures || !this.canManage) return;

		// Find the feature by key
		const feature = this.earlyAdopterFeatures.find(f => f.key === featureKey);
		if (!feature) {
			console.error('Feature not found:', featureKey);
			return;
		}

		// Store the current state before toggling
		const currentState = feature.enabled;
		const newState = !currentState;
		const featureName = feature.name;

		this.isUpdatingFeatures = true;
		try {
			const featureFlags: { [key: string]: boolean } = {};
			featureFlags[featureKey] = newState;

			await this.settingService.updateOrganisationFeatureFlags({
				org_id: this.organisationId,
				body: { feature_flags: featureFlags }
			});

			// Update the feature state
			feature.enabled = newState;
			const action = feature.enabled ? 'enabled' : 'disabled';
			const message = `${featureName} has been ${action}`;
			this.generalService.showNotification({ style: 'success', message });
			this.isUpdatingFeatures = false;
		} catch (error) {
			console.error('Error toggling feature:', error);
			this.isUpdatingFeatures = false;
		}
	}
}

