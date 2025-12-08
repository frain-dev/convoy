import { Component, OnInit } from '@angular/core';
import { AdminService } from '../admin.service';
import { GeneralService } from 'src/app/services/general/general.service';

interface FeatureFlag {
	uid: string;
	feature_key: string;
	enabled: boolean;
	created_at?: string;
	updated_at?: string;
}

@Component({
	selector: 'app-feature-flags',
	templateUrl: './feature-flags.component.html',
	styleUrls: ['./feature-flags.component.scss']
})
export class FeatureFlagsComponent implements OnInit {
	featureFlags: FeatureFlag[] = [];
	isLoadingFeatureFlags = false;
	isUpdatingFeatureFlag = false;

	constructor(
		private adminService: AdminService,
		private generalService: GeneralService
	) {}

	async ngOnInit() {
		await this.loadFeatureFlags();
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

	trackByFeatureKey(index: number, flag: FeatureFlag): string {
		return flag.feature_key;
	}

	async toggleFeatureFlag(featureFlag: FeatureFlag, event: Event) {
		const input = event?.target as HTMLInputElement | null;
		if (!input) {
			console.error('Toggle event missing target', event);
			return;
		}

		const enabled = input.checked;
		const previousValue = featureFlag.enabled;

		// Optimistically update UI
		featureFlag.enabled = enabled;

		this.isUpdatingFeatureFlag = true;
		try {
			const response = await this.adminService.updateFeatureFlag(featureFlag.feature_key, enabled);
			// Update with server response to ensure consistency
			if (response.data) {
				const index = this.featureFlags.findIndex(f => f.feature_key === featureFlag.feature_key);
				if (index !== -1) {
					this.featureFlags[index] = { ...this.featureFlags[index], ...response.data };
				}
			}
			this.generalService.showNotification({ style: 'success', message: 'Feature flag updated successfully' });
		} catch (error) {
			console.error('Error updating feature flag:', error);
			// Revert on error
			featureFlag.enabled = previousValue;
			input.checked = previousValue;
			this.generalService.showNotification({ style: 'error', message: 'Failed to update feature flag' });
		} finally {
			this.isUpdatingFeatureFlag = false;
		}
	}

}
