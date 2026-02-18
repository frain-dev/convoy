import {Component, OnInit} from '@angular/core';
import {ActivatedRoute, Router} from '@angular/router';
import {LicensesService} from 'src/app/services/licenses/licenses.service';
import {HttpService} from 'src/app/services/http/http.service';
import {RbacService} from 'src/app/services/rbac/rbac.service';
import {GeneralService} from 'src/app/services/general/general.service';
import {CheckoutResolverData} from './billing/checkout.resolver';

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
	isVerifyingSubscription = false;
	settingsMenu: { name: SETTINGS; icon: string; svg: 'stroke' | 'fill' }[] = [
		{ name: 'organisation settings', icon: 'org', svg: 'fill' },
		{ name: 'team', icon: 'team', svg: 'stroke' }
	];

	constructor(
		private router: Router,
		private route: ActivatedRoute,
		public licenseService: LicensesService,
		private httpService: HttpService,
		private rbacService: RbacService,
		private generalService: GeneralService
	) {}

	async ngOnInit() {
		await this.checkBillingAccess();
		this.checkBillingStatus();

		const checkoutData: CheckoutResolverData = this.route.snapshot.data['checkout'];
		if (checkoutData?.checkoutProcessed) {
			this.cleanupCheckoutParams();
		} else if (checkoutData?.needsPolling) {
			this.pollSubscriptionStatus(checkoutData.orgId);
		}

		const requestedPage = this.route.snapshot.queryParams?.activePage ?? 'organisation settings';
		this.setActivePageWithLicenseCheck(requestedPage);
	}
	
	private async pollSubscriptionStatus(orgId: string) {
		this.isVerifyingSubscription = true;
		const maxAttempts = 30;
		const pollInterval = 2000;

		for (let i = 0; i < maxAttempts; i++) {
			await new Promise(r => setTimeout(r, pollInterval));
			try {
				const response = await this.httpService.request({
					url: `/billing/organisations/${orgId}/subscription`,
					method: 'get',
					hideNotification: true
				});
				if (response.data?.status === 'active') {
					this.generalService.showNotification({ message: 'Subscription activated successfully!', style: 'success' });
					this.isVerifyingSubscription = false;
					this.cleanupCheckoutParams();
					return;
				}
			} catch (_) {}
		}

		this.generalService.showNotification({ message: 'Unable to verify subscription. Please check billing page.', style: 'warning' });
		this.isVerifyingSubscription = false;
		this.cleanupCheckoutParams();
	}

	private cleanupCheckoutParams() {
		const activePage = this.route.snapshot.queryParams?.['activePage'] || 'usage and billing';
		this.router.navigate([], {
			relativeTo: this.route,
			queryParams: { activePage },
			replaceUrl: true
		});
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
		this.router.navigate([], { 
			relativeTo: this.route,
			queryParams: { activePage: this.activePage }
		});
	}

	getUserLimitMessage(): string {
		if (!this.licenseService.hasLicense('user_limit')) {
			if (!this.licenseService.isLimitAvailable('user_limit')) {
				return 'Business';
			}
			if (this.licenseService.isLimitAvailable('user_limit') && this.licenseService.isLimitReached('user_limit')) {
				const limitInfo = this.licenseService.getLimitInfo('user_limit');
				const current = limitInfo?.current ?? 0;
				const limit = limitInfo?.limit === -1 ? '∞' : (limitInfo?.limit ?? 0);
				return `Limit reached (${current}/${limit})`;
			}
		}
		return '';
	}
}
