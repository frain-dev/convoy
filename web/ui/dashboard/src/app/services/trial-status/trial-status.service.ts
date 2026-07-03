import { Injectable } from '@angular/core';
import { BehaviorSubject, Observable } from 'rxjs';
import { HttpService } from '../http/http.service';
import { OrganisationStateService } from '../organisation-state/organisation-state.service';
import { BillingStrategy } from '../../models/billing.model';
import { BillingEndpoints } from '../../private/pages/settings/billing/billing-endpoints';
import { TrialOffer } from '../../private/pages/settings/billing/trial-offer.util';

export type { TrialOffer, TrialOfferLimit } from '../../private/pages/settings/billing/trial-offer.util';

export interface TrialStatus {
	// Ready-to-render label for the nav pill. Deliberately static ("Trial"): the
	// trial has no user-facing countdown, since at trial end the subscription is
	// either converted (card on file) or cancelled automatically (no card).
	label: string;
}

// Global trial awareness for the app shell.
@Injectable({ providedIn: 'root' })
export class TrialStatusService {
	private state$ = new BehaviorSubject<TrialStatus | null>(null);
	readonly status$: Observable<TrialStatus | null> = this.state$.asObservable();

	// Whether the current org may still START a trial (never trialed, no
	// subscription). Distinct from status$ (whether it is ON a trial). Defaults
	// to false so the UI never offers a trial before eligibility is confirmed.
	private eligibleState$ = new BehaviorSubject<boolean>(false);
	readonly eligible$: Observable<boolean> = this.eligibleState$.asObservable();

	// Trial terms/limits from the org billing payload, for the "Start trial"
	// modal. Null until resolved or when the org is not on cloud.
	private offerState$ = new BehaviorSubject<TrialOffer | null>(null);
	readonly offer$: Observable<TrialOffer | null> = this.offerState$.asObservable();

	constructor(private httpService: HttpService, private orgState: OrganisationStateService) {}

	// Refresh trial state (pill) and trial eligibility for the current org.
	// Fail-closed: missing org, unsupported strategy, or request error clears
	// the pill and marks the org ineligible rather than guessing.
	async refresh(): Promise<void> {
		const strategy = await this.orgState.getBillingStrategy();
		const orgId = this.httpService.getOrganisation()?.uid;
		if (!orgId) {
			this.state$.next(null);
			this.eligibleState$.next(false);
			this.offerState$.next(null);
			return;
		}

		if (strategy === 'cloud') {
			await Promise.all([this.refreshTrialStatus(strategy, orgId), this.refreshEligibility(strategy, orgId)]);
			return;
		}

		if (strategy === 'licensed_self_hosted') {
			await this.refreshTrialStatus(strategy, orgId);
			this.eligibleState$.next(false);
			this.offerState$.next(null);
			return;
		}

		this.state$.next(null);
		this.eligibleState$.next(false);
		this.offerState$.next(null);
	}

	private async refreshTrialStatus(strategy: BillingStrategy, orgId: string): Promise<void> {
		try {
			const response = await this.httpService.request({
				url: BillingEndpoints.billingUrl(strategy, 'subscription', orgId),
				method: 'get',
				hideNotification: true
			});

			const subscription = response?.data;
			if (subscription?.trial === true) {
				this.state$.next({ label: 'Trial' });
			} else {
				this.state$.next(null);
			}
		} catch {
			this.state$.next(null);
		}
	}

	// Eligibility comes from the org billing payload (trial_eligible), which is
	// present even when the org has no subscription, so it distinguishes a
	// never-trialed org from one that already trialed and cancelled (both of
	// which have no active subscription).
	private async refreshEligibility(strategy: 'cloud', orgId: string): Promise<void> {
		try {
			const response = await this.httpService.request({
				url: BillingEndpoints.billingUrl(strategy, 'organisation', orgId),
				method: 'get',
				hideNotification: true
			});

			this.eligibleState$.next(response?.data?.trial_eligible === true);
			this.offerState$.next(response?.data?.trial_offer ?? null);
		} catch {
			this.eligibleState$.next(false);
			this.offerState$.next(null);
		}
	}

	clear(): void {
		this.state$.next(null);
		this.eligibleState$.next(false);
		this.offerState$.next(null);
	}
}
