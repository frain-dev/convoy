import { Component, OnInit } from '@angular/core';
import { FormBuilder, FormGroup } from '@angular/forms';
import { AdminService } from '../admin.service';
import { GeneralService } from 'src/app/services/general/general.service';

interface FeatureFlag {
	uid: string;
	feature_key: string;
	enabled: boolean;
	allow_override: boolean;
	created_at?: string;
	updated_at?: string;
}

interface Organisation {
	uid: string;
	name: string;
	created_at?: string;
	updated_at?: string;
}

interface FeatureFlagOverride {
	uid: string;
	feature_flag_id: string;
	feature_key?: string;
	owner_type: string;
	owner_id: string;
	enabled: boolean;
	enabled_at?: string;
	enabled_by?: string;
	created_at?: string;
	updated_at?: string;
}

@Component({
	selector: 'app-organisation-overrides',
	templateUrl: './organisation-overrides.component.html',
	styleUrls: ['./organisation-overrides.component.scss']
})
export class OrganisationOverridesComponent implements OnInit {
	featureFlags: FeatureFlag[] = [];
	organisations: Organisation[] = [];
	selectedOrganisation: Organisation | null = null;
	organisationOverrides: Map<string, FeatureFlagOverride> = new Map();
	isLoadingFeatureFlags = false;
	isLoadingOrganisations = false;
	isLoadingOverrides = false;
	isUpdatingOverride = false;
	organisationForm: FormGroup;
	organisationSearchTerm = '';
	private searchTimeout: any;

	constructor(
		private adminService: AdminService,
		private generalService: GeneralService,
		private formBuilder: FormBuilder
	) {
		this.organisationForm = this.formBuilder.group({
			organisation: [null]
		});
	}

	async ngOnInit() {
		await Promise.all([this.loadFeatureFlags(), this.loadOrganisations()]);
	}

	async loadFeatureFlags() {
		this.isLoadingFeatureFlags = true;
		try {
			const response = await this.adminService.getAllFeatureFlags();
			const allFlags: FeatureFlag[] = response.data || [];
			// Filter out credential-encryption (system-level only, like prometheus) and full-text-search (deprecated)
			this.featureFlags = allFlags.filter(flag => 
				flag.feature_key !== 'credential-encryption' && 
				flag.feature_key !== 'full-text-search'
			);
		} catch (error) {
			console.error('Error loading feature flags:', error);
			this.generalService.showNotification({ style: 'error', message: 'Failed to load feature flags' });
		} finally {
			this.isLoadingFeatureFlags = false;
		}
	}

	async loadOrganisations(searchTerm?: string) {
		this.isLoadingOrganisations = true;
		try {
			const response = await this.adminService.getAllOrganisations({ 
				page: 1, 
				perPage: 1000,
				search: searchTerm || ''
			});
			this.organisations = response.data?.content || [];
		} catch (error) {
			console.error('Error loading organisations:', error);
			this.generalService.showNotification({ style: 'error', message: 'Failed to load organisations' });
		} finally {
			this.isLoadingOrganisations = false;
		}
	}

	filterOrganisations(searchTerm: string) {
		this.organisationSearchTerm = searchTerm;
		
		if (this.searchTimeout) {
			clearTimeout(this.searchTimeout);
		}

		this.searchTimeout = setTimeout(() => {
			this.loadOrganisations(searchTerm.trim());
		}, 500);
	}

	async selectOrganisation(org: Organisation) {
		if (!org || !org.uid) {
			console.error('Invalid organisation:', org);
			return;
		}
		this.selectedOrganisation = org;
		await this.loadOrganisationOverrides(org.uid);
	}

	async loadOrganisationOverrides(orgID: string) {
		this.isLoadingOverrides = true;
		this.organisationOverrides.clear();
		try {
			const response = await this.adminService.getOrganisationOverrides(orgID);
			const overrides: FeatureFlagOverride[] = response.data || [];
			overrides.forEach(override => {
				this.organisationOverrides.set(override.feature_flag_id, override);
			});
		} catch (error) {
			console.error('Error loading organisation overrides:', error);
			this.generalService.showNotification({ style: 'error', message: 'Failed to load organisation overrides' });
		} finally {
			this.isLoadingOverrides = false;
		}
	}

	getOverrideForFeatureFlag(featureFlag: FeatureFlag): FeatureFlagOverride | null {
		if (!this.selectedOrganisation) return null;
		return this.organisationOverrides.get(featureFlag.uid) || null;
	}

	isFeatureFlagOverridden(featureFlag: FeatureFlag): boolean {
		return this.getOverrideForFeatureFlag(featureFlag) !== null;
	}

	getEffectiveState(featureFlag: FeatureFlag): boolean {
		const override = this.getOverrideForFeatureFlag(featureFlag);
		return override ? override.enabled : featureFlag.enabled;
	}

	async toggleOverride(featureFlag: FeatureFlag, event: Event) {
		if (!this.selectedOrganisation || !featureFlag.allow_override) return;

		const input = event?.target as HTMLInputElement | null;
		if (!input) {
			console.error('Toggle event missing target', event);
			return;
		}

		const enabled = input.checked;
		const previousOverride = this.organisationOverrides.get(featureFlag.uid);

		// Optimistically update UI
		if (enabled) {
			const newOverride: FeatureFlagOverride = {
				uid: previousOverride?.uid || '',
				feature_flag_id: featureFlag.uid,
				owner_type: 'organisation',
				owner_id: this.selectedOrganisation.uid,
				enabled: true,
				...previousOverride
			};
			this.organisationOverrides.set(featureFlag.uid, newOverride);
		} else {
			// If disabling, we'll remove it after successful API call
		}

		this.isUpdatingOverride = true;
		try {
			const response = await this.adminService.updateOrganisationOverride(
				this.selectedOrganisation.uid,
				featureFlag.feature_key,
				enabled
			);
			// Update with server response
			if (response.data) {
				this.organisationOverrides.set(featureFlag.uid, response.data);
			} else if (!enabled) {
				// Remove override if disabled
				this.organisationOverrides.delete(featureFlag.uid);
			}
			this.generalService.showNotification({ style: 'success', message: 'Override updated successfully' });
		} catch (error) {
			console.error('Error updating override:', error);
			// Revert on error
			if (previousOverride) {
				this.organisationOverrides.set(featureFlag.uid, previousOverride);
			} else {
				this.organisationOverrides.delete(featureFlag.uid);
			}
			input.checked = !enabled;
			this.generalService.showNotification({ style: 'error', message: 'Failed to update override' });
		} finally {
			this.isUpdatingOverride = false;
		}
	}

	async removeOverride(featureFlag: FeatureFlag) {
		if (!this.selectedOrganisation) return;

		const previousOverride = this.organisationOverrides.get(featureFlag.uid);

		// Optimistically remove from UI
		this.organisationOverrides.delete(featureFlag.uid);

		this.isUpdatingOverride = true;
		try {
			await this.adminService.deleteOrganisationOverride(
				this.selectedOrganisation.uid,
				featureFlag.feature_key
			);
			this.generalService.showNotification({ style: 'success', message: 'Override removed successfully' });
		} catch (error) {
			console.error('Error removing override:', error);
			// Revert on error
			if (previousOverride) {
				this.organisationOverrides.set(featureFlag.uid, previousOverride);
			}
			this.generalService.showNotification({ style: 'error', message: 'Failed to remove override' });
		} finally {
			this.isUpdatingOverride = false;
		}
	}

	getFeatureFlagDisplayName(key: string): string {
		const names: { [key: string]: string } = {
			'ip-rules': 'IP Rules',
			'prometheus': 'Prometheus',
			'circuit-breaker': 'Circuit Breaker',
			'retention-policy': 'Retention Policy',
			'read-replicas': 'Read Replicas',
			'mtls': 'mTLS',
			'oauth-token-exchange': 'OAuth Token Exchange'
		};
		return names[key] || key;
	}

	getOrganisationOptions(): Array<{ uid: string; name: string }> {
		return this.organisations.map(org => ({ 
			uid: org.uid, 
			name: `${org.name} (${org.uid})` 
		}));
	}

	trackByFeatureKey(index: number, flag: FeatureFlag): string {
		return flag.feature_key;
	}

	onOrganisationSelected(selectedOrg: any) {
		const orgUid = typeof selectedOrg === 'string' ? selectedOrg : selectedOrg?.uid;
		if (!orgUid) return;
		
		const org = this.organisations.find(o => o.uid === orgUid);
		if (org) {
			// Update form control to the selected option object so dropdown shows the name
			this.organisationForm.patchValue({ organisation: { uid: org.uid, name: `${org.name} (${org.uid})` } });
			this.selectOrganisation(org);
		}
	}
}
