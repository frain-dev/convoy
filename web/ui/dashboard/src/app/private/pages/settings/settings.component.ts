import {Component, OnInit} from '@angular/core';
import {ActivatedRoute, Router} from '@angular/router';
import {LicensesService} from 'src/app/services/licenses/licenses.service';
import {HttpService} from 'src/app/services/http/http.service';
import {RbacService} from 'src/app/services/rbac/rbac.service';
import {GeneralService} from 'src/app/services/general/general.service';
import {BillingPaymentDetailsService} from './billing/billing-payment-details.service';
import {CheckoutResolverData} from './billing/checkout.resolver';

export type SETTINGS = 'organisation settings' | 'configuration settings' | 'personal access tokens' | 'team' | 'usage and billing' | 'early adopter features';

@Component({
    selector: 'convoy-settings',
    templateUrl: './settings.component.html',
    styleUrls: ['./settings.component.scss'],
    standalone: false
})
export class SettingsComponent implements OnInit {
	activePage: SETTINGS = 'organisation settings';
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
		private generalService: GeneralService,
		private billingPaymentDetailsService: BillingPaymentDetailsService
	) {}

	async ngOnInit() {
		await this.checkBillingAccess();
		this.updateSettingsMenu();

		const checkoutData: CheckoutResolverData = this.route.snapshot.data['checkout'];
		if (checkoutData?.checkoutProcessed) {
			this.setActivePageWithLicenseCheck('usage and billing');
			this.cleanupCheckoutParams();
		} else if (checkoutData?.token || checkoutData?.attemptId) {
			this.setActivePageWithLicenseCheck('usage and billing');
			this.cleanupCheckoutParams();
			this.completeSelfHostedCheckout(checkoutData.token, checkoutData.attemptId);
		} else if (checkoutData?.needsPolling) {
			this.setActivePageWithLicenseCheck('usage and billing');
			this.pollSubscriptionStatus(checkoutData.orgId);
		}

		if (!checkoutData?.checkoutProcessed && !checkoutData?.token && !checkoutData?.attemptId && !checkoutData?.needsPolling) {
			const requestedPage = this.route.snapshot.queryParams?.activePage ?? 'organisation settings';
			this.setActivePageWithLicenseCheck(requestedPage);
		}
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
					await this.licenseService.loadAllLicenses();
					this.billingPaymentDetailsService.notifyCheckoutSubscriptionVerified();
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

	private async completeSelfHostedCheckout(token: string, attemptId: string) {
		this.isVerifyingSubscription = true;
		const maxAttempts = 30;
		const pollInterval = 2000;

		for (let i = 0; i < maxAttempts; i++) {
			try {
				const response = await this.httpService.request({
					url: '/billing/sh_checkout/complete',
					method: 'post',
					body: { token, attempt_id: attemptId },
					hideNotification: true
				});

				if (response.data?.status === 'completed') {
					if (attemptId) localStorage.setItem(`checkout_processed_${attemptId}`, 'true');
					await this.licenseService.loadAllLicenses();
					this.generalService.showNotification({ message: 'License activated successfully!', style: 'success' });
					this.isVerifyingSubscription = false;
					this.cleanupCheckoutParams();
					return;
				}
			} catch (_) {}

			await new Promise(r => setTimeout(r, pollInterval));
		}

		this.generalService.showNotification({ message: 'Payment is still pending. You can resume checkout from Usage and Billing shortly.', style: 'warning' });
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

	private async checkBillingAccess() {
		try {
			const userRole = await this.rbacService.getUserRole();
			// Instance admins need billing access for the self-hosted instance license flow;
			// cloud org billing still relies on the backend's org-scoped billing checks.
			this.canAccessBilling = userRole === 'BILLING_ADMIN' || userRole === 'ORGANISATION_ADMIN' || userRole === 'INSTANCE_ADMIN';
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

		if (this.canAccessBilling) {
			this.settingsMenu.push({ name: 'usage and billing', icon: 'status', svg: 'stroke' });
		}

		if (this.canAccessEarlyAdopterFeatures) {
			this.settingsMenu.push({ name: 'early adopter features', icon: 'settings', svg: 'fill' });
		}

		if (this.activePage === 'usage and billing' && !this.canAccessBilling) {
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
