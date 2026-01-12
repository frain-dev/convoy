import {Component, OnInit} from '@angular/core';
import {ActivatedRoute, Router} from '@angular/router';
import {LicensesService} from 'src/app/services/licenses/licenses.service';
import {HttpService} from 'src/app/services/http/http.service';
import {RbacService} from 'src/app/services/rbac/rbac.service';

export type SETTINGS = 'organisation settings' | 'configuration settings' | 'personal access tokens' | 'team' | 'usage and billing' | 'early adopter features';

@Component({
	selector: 'convoy-settings',
	templateUrl: './settings.component.html',
	styleUrls: ['./settings.component.scss']
})
export class SettingsComponent implements OnInit {
	activePage: SETTINGS = 'organisation settings';
	billingEnabled = false;
	canAccessBilling = false;
	canAccessEarlyAdopterFeatures = false;
	settingsMenu: { name: SETTINGS; icon: string; svg: 'stroke' | 'fill' }[] = [
		{ name: 'organisation settings', icon: 'org', svg: 'fill' },
		{ name: 'team', icon: 'team', svg: 'stroke' }
		// { name: 'configuration settings', icon: 'settings', svg: 'fill' }
	];

	constructor(
		private router: Router,
		private route: ActivatedRoute,
		public licenseService: LicensesService,
		private httpService: HttpService,
		private rbacService: RbacService
	) {}

	async ngOnInit() {
		await this.checkBillingAccess();
		this.checkBillingStatus();
		// Set active page from URL query parameter with license validation
		const requestedPage = this.route.snapshot.queryParams?.activePage ?? 'organisation settings';
		this.setActivePageWithLicenseCheck(requestedPage);
	}

	private async checkBillingStatus() {
		try {
			const response = await this.httpService.request({
				url: '/billing/enabled',
				method: 'get',
				hideNotification: true
			});
			// Billing is now controlled by backend configuration, not entitlements
			this.billingEnabled = response.data?.enabled || false;
			this.updateSettingsMenu();
		} catch (error) {
			console.warn('Failed to check billing status:', error);
			this.billingEnabled = false;
			this.updateSettingsMenu();
		}
	}

	private async checkBillingAccess() {
		try {
			const userRole = await this.rbacService.getUserRole();
			this.canAccessBilling = userRole === 'BILLING_ADMIN' || userRole === 'ORGANISATION_ADMIN';
			this.canAccessEarlyAdopterFeatures = true;
		} catch (error) {
			console.warn('Failed to check billing access:', error);
			this.canAccessBilling = false;
			this.canAccessEarlyAdopterFeatures = false;
		}
	}

	private updateSettingsMenu() {
		this.settingsMenu = [
			{ name: 'organisation settings', icon: 'org', svg: 'fill' },
			{ name: 'team', icon: 'team', svg: 'stroke' }
		];

		if (this.billingEnabled && this.canAccessBilling) {
			this.settingsMenu.push({ name: 'usage and billing', icon: 'status', svg: 'stroke' });
		}

		if (this.canAccessEarlyAdopterFeatures) {
			this.settingsMenu.push({ name: 'early adopter features', icon: 'settings', svg: 'fill' });
		}

		if (this.activePage === 'usage and billing' && (!this.billingEnabled || !this.canAccessBilling)) {
			this.toggleActivePage('organisation settings');
		}

		if (this.activePage === 'early adopter features' && !this.canAccessEarlyAdopterFeatures) {
			this.toggleActivePage('organisation settings');
		}
	}

	setActivePageWithLicenseCheck(requestedPage: string) {
		// CREATE_USER entitlement removed - user limits handle this now
		// Team page access is now controlled by user limits on the backend

		if (requestedPage === 'usage and billing' && !this.canAccessBilling) {
			this.toggleActivePage('organisation settings');
			return;
		}

		this.toggleActivePage(requestedPage as SETTINGS);
	}

	toggleActivePage(activePage: SETTINGS) {
		this.activePage = activePage;
		if (!this.router.url.split('/')[2]) this.addPageToUrl();
	}

	addPageToUrl() {
		const queryParams: any = {};
		queryParams.activePage = this.activePage;
		this.router.navigate([], { queryParams: Object.assign({}, queryParams) });
	}
}
