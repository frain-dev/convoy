import {Component, OnDestroy, OnInit} from '@angular/core';
import {ActivatedRoute, Router} from '@angular/router';
import {Subscription} from 'rxjs';
import {LicensesService} from 'src/app/services/licenses/licenses.service';
import {HttpService} from 'src/app/services/http/http.service';
import {RbacService} from 'src/app/services/rbac/rbac.service';
import {GeneralService} from 'src/app/services/general/general.service';
import {BillingPaymentDetailsService} from './billing/billing-payment-details.service';
import {CheckoutResolverData} from './billing/checkout.resolver';
import {pollUntil, CHECKOUT_POLL_BUDGET_MS} from 'src/app/utils/poll.util';
import {CHECKOUT_STATUS} from 'src/app/models/billing.model';

export const SETTINGS_PAGE = {
	ORGANISATION: 'organisation settings',
	CONFIGURATION: 'configuration settings',
	TOKENS: 'personal access tokens',
	TEAM: 'team',
	BILLING: 'usage and billing',
	EARLY_ADOPTER: 'early adopter features'
} as const;

export type SETTINGS = typeof SETTINGS_PAGE[keyof typeof SETTINGS_PAGE];

@Component({
    selector: 'convoy-settings',
    templateUrl: './settings.component.html',
    styleUrls: ['./settings.component.scss'],
    standalone: false
})
export class SettingsComponent implements OnInit, OnDestroy {
	activePage: SETTINGS = SETTINGS_PAGE.ORGANISATION;
	canAccessBilling = false;
	canAccessEarlyAdopterFeatures = false;
	settingsMenu: { name: SETTINGS; icon: string; svg: 'stroke' | 'fill' }[] = [
		{ name: SETTINGS_PAGE.ORGANISATION, icon: 'org', svg: 'fill' },
		{ name: SETTINGS_PAGE.TEAM, icon: 'team', svg: 'stroke' }
	];

	private queryParamsSub?: Subscription;

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
			this.setActivePageWithLicenseCheck(SETTINGS_PAGE.BILLING);
			this.cleanupCheckoutParams();
		} else if (checkoutData?.token || checkoutData?.attemptId) {
			this.setActivePageWithLicenseCheck(SETTINGS_PAGE.BILLING);
			this.cleanupCheckoutParams();
			this.completeSelfHostedCheckout(checkoutData.token, checkoutData.attemptId);
		} else if (checkoutData?.needsPolling) {
			this.setActivePageWithLicenseCheck(SETTINGS_PAGE.BILLING);
			this.pollSubscriptionStatus(checkoutData.orgId);
		}

		if (!checkoutData?.checkoutProcessed && !checkoutData?.token && !checkoutData?.attemptId && !checkoutData?.needsPolling) {
			const requestedPage = this.route.snapshot.queryParams?.activePage ?? SETTINGS_PAGE.ORGANISATION;
			this.setActivePageWithLicenseCheck(requestedPage);
		}

		this.queryParamsSub = this.route.queryParams.subscribe((params) => {
			const requestedPage = params['activePage'];
			if (!requestedPage || requestedPage === this.activePage) {
				return;
			}
			this.setActivePageWithLicenseCheck(requestedPage);
		});
	}

	ngOnDestroy(): void {
		this.queryParamsSub?.unsubscribe();
	}

	private async pollSubscriptionStatus(orgId: string) {
		this.billingPaymentDetailsService.notifyCheckoutVerificationStarted();

		const verified = await pollUntil({
			budgetMs: CHECKOUT_POLL_BUDGET_MS,
			delayFirst: true,
			request: () => this.httpService.request({
				url: `/billing/organisations/${orgId}/subscription`,
				method: 'get',
				hideNotification: true
			}),
			isDone: (response: any) => response.data?.status === CHECKOUT_STATUS.ACTIVE
		});

		if (verified) {
			await this.licenseService.loadAllLicenses();
			this.billingPaymentDetailsService.notifyCheckoutSubscriptionVerified();
			this.generalService.showNotification({ message: 'Subscription activated successfully!', style: 'success' });
			this.cleanupCheckoutParams();
			return;
		}

		this.generalService.showNotification({ message: 'Unable to verify subscription. Please check billing page.', style: 'warning' });
		this.cleanupCheckoutParams();
	}

	private async completeSelfHostedCheckout(token: string, attemptId: string) {
		this.billingPaymentDetailsService.notifyCheckoutVerificationStarted();

		const completed = await pollUntil({
			budgetMs: CHECKOUT_POLL_BUDGET_MS,
			request: () => this.httpService.request({
				url: '/billing/sh_checkout/complete',
				method: 'post',
				body: { token, attempt_id: attemptId },
				hideNotification: true
			}),
			isDone: (response: any) => response.data?.status === CHECKOUT_STATUS.COMPLETED
		});

		if (completed) {
			if (attemptId) localStorage.setItem(`checkout_processed_${attemptId}`, 'true');
			await this.licenseService.loadAllLicenses();
			this.billingPaymentDetailsService.notifyCheckoutSubscriptionVerified();
			this.generalService.showNotification({ message: 'License activated successfully!', style: 'success' });
			this.cleanupCheckoutParams();
			return;
		}

		this.generalService.showNotification({ message: 'Payment is still pending. You can resume checkout from Usage and Billing shortly.', style: 'warning' });
		this.cleanupCheckoutParams();
	}

	private cleanupCheckoutParams() {
		const activePage = this.route.snapshot.queryParams?.['activePage'] || SETTINGS_PAGE.BILLING;
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
			{ name: SETTINGS_PAGE.ORGANISATION, icon: 'org', svg: 'fill' },
			{ name: SETTINGS_PAGE.TEAM, icon: 'team', svg: 'stroke' }
		];

		if (this.canAccessBilling) {
			this.settingsMenu.push({ name: SETTINGS_PAGE.BILLING, icon: 'status', svg: 'stroke' });
		}

		if (this.canAccessEarlyAdopterFeatures) {
			this.settingsMenu.push({ name: SETTINGS_PAGE.EARLY_ADOPTER, icon: 'settings', svg: 'fill' });
		}

		if (this.activePage === SETTINGS_PAGE.BILLING && !this.canAccessBilling) {
			this.toggleActivePage(SETTINGS_PAGE.ORGANISATION);
		}

		if (this.activePage === SETTINGS_PAGE.EARLY_ADOPTER && !this.canAccessEarlyAdopterFeatures) {
			this.toggleActivePage(SETTINGS_PAGE.ORGANISATION);
		}
	}

	setActivePageWithLicenseCheck(requestedPage: string) {
		// CREATE_USER entitlement removed - user limits handle this now
		// Team page access is now controlled by user limits on the backend

		if (requestedPage === SETTINGS_PAGE.BILLING && !this.canAccessBilling) {
			this.toggleActivePage(SETTINGS_PAGE.ORGANISATION);
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
		return this.licenseService.limitMessage('user_limit');
	}

	// Compact pill text so the tag fits the 200px sidebar: "1/1" for a reached limit,
	// the upsell label unchanged. Full message stays available as the tooltip.
	getUserLimitPillText(): string {
		return this.licenseService.limitPillText('user_limit');
	}
}
