import { Component, OnDestroy, OnInit, ViewChild } from '@angular/core';
import { Subscription } from 'rxjs';
import { PROJECT } from 'src/app/models/project.model';
import { PrivateService } from '../../private.service';
import { Router } from '@angular/router';
import { LicensesService } from 'src/app/services/licenses/licenses.service';
import { OrganisationStateService } from 'src/app/services/organisation-state/organisation-state.service';
import { BillingStrategy, SelfHostedBillingConfig } from 'src/app/models/billing.model';
import { GeneralService } from 'src/app/services/general/general.service';
import { TrialStatusService } from 'src/app/services/trial-status/trial-status.service';
import { TrialModalComponent } from '../settings/billing/trial-modal.component';
import { BillingPaymentDetailsService } from '../settings/billing/billing-payment-details.service';
import {
	canBlockProjectsWithCloudTrial,
	canOfferProjectTrial,
	canShowSelfHostedTrialUpsell,
	canStartSelfHostedTrial as evaluateSelfHostedTrialEligibility,
	formatTrialIntro,
	formatCloudTrialUpsellLead,
	formatSelfHostedTrialUpsellLead,
	resolveSelfHostedTrialOffer,
	resolveTrialModalMode,
	TrialOffer
} from '../settings/billing/trial-offer.util';
import { pollUntil, POLL_BUDGET_MS } from 'src/app/utils/poll.util';

@Component({
    selector: 'app-projects',
    templateUrl: './projects.component.html',
    styleUrls: ['./projects.component.scss'],
    standalone: false
})
export class ProjectsComponent implements OnInit, OnDestroy {
	projects: PROJECT[] = [];
	isLoadingProjects = false;
	projectsLoaderIndex: number[] = [0, 1, 2, 3, 4];
	showOrganisationModal = false;
	isLoadingProject: boolean = false;
	billingStrategy: BillingStrategy = 'oss';
	isStartingTrial = false;
	// True from a successful trial start until project creation unlocks. Keeps the
	// empty state on a "setting up your trial" message instead of reverting to the
	// "Start trial" button (which reads as failure) while the entitlement webhook
	// propagates.
	trialProvisioning = false;
	// Whether this org may still start a trial (from the billing service, via
	// TrialStatusService). Defaults false so we never offer a trial before it is
	// confirmed, and so an already-trialed org (e.g. after cancelling) stops
	// seeing the CTA.
	trialEligible = false;
	cloudTrialOffer: TrialOffer | null = null;
	selfHostedBillingConfig: SelfHostedBillingConfig | null = null;
	billingConfigLoaded = false;
	// False until billing strategy, licenses, and trial eligibility have all
	// resolved for this org. The empty-state CTA stays on a loader until then so
	// a fresh org never flashes the "upgrade / Go to Billing" state before its
	// trial eligibility lands (which previously only corrected after navigating
	// away and back).
	ctaResolved = false;
	private trialEligibleSub?: Subscription;
	private trialOfferSub?: Subscription;
	private trialPollToken = 0;
	@ViewChild('trialModal') trialModal!: TrialModalComponent;

	constructor(
		private privateService: PrivateService,
		private router: Router,
		public licenseService: LicensesService,
		private orgState: OrganisationStateService,
		private generalService: GeneralService,
		private trialStatusService: TrialStatusService,
		private billingPaymentDetailsService: BillingPaymentDetailsService
	) {
		this.privateService.projects$.subscribe(projects => (this.projects = projects.data));
	}

	async ngOnInit() {
		// Subscribe first so we pick up the latest eligibility and any org switch.
		this.trialEligibleSub = this.trialStatusService.eligible$.subscribe(eligible => (this.trialEligible = eligible));
		this.trialOfferSub = this.trialStatusService.offer$.subscribe(offer => (this.cloudTrialOffer = offer));
		await this.getProjects();
		// Org is now present (projects loaded), so resolve the trial CTA
		// deterministically before revealing it. Awaiting here (rather than a
		// fire-and-forget refresh) avoids the fresh-org race where eligibility
		// lands after the empty state has already rendered the upgrade message.
		await this.resolveTrialCta();
	}

	// Resolve everything the empty-state CTA depends on (trial eligibility and the
	// license cache that drives shouldBlockProjectCreation), then reveal the CTA.
	// Always reveals in the finally so a failed request shows the normal CTA
	// rather than hanging on the loader.
	private async resolveTrialCta() {
		try {
			await Promise.all([
				this.trialStatusService.refresh(),
				this.licenseService.loadAllLicenses(),
				this.loadBillingContext()
			]);
		} finally {
			this.ctaResolved = true;
		}
	}

	private loadBillingContext(): Promise<void> {
		return new Promise(resolve => {
			this.billingPaymentDetailsService.getBillingConfig().subscribe({
				next: (config) => {
					this.billingStrategy = config.data.strategy || 'oss';
					this.selfHostedBillingConfig = config.data.self_hosted || null;
					this.billingConfigLoaded = true;
					resolve();
				},
				error: () => {
					this.billingConfigLoaded = true;
					resolve();
				}
			});
		});
	}

	async getProject(projectId: string) {
		this.isLoadingProjects = true;

		try {
			await this.privateService.getProject({ refresh: true, projectId });
			await this.privateService.getProjectStat({ refresh: true });

			this.router.navigate([`/projects/${projectId}`]);
			this.isLoadingProjects = false;
		} catch (error) {
			this.isLoadingProjects = false;
		}
	}

	async getProjects(): Promise<any> {
		this.isLoadingProjects = true;

		try {
			const response = await this.privateService.getProjects();
			this.projects = response.data;
			this.isLoadingProjects = false;
		} catch (error) {
			this.isLoadingProjects = false;
			return error;
		}
	}

	get isDisabled(): boolean {
		return this.orgState.isDisabled();
	}

	/** Message for the card layout (grid) overlay. */
	getProjectLimitMessage(): string {
		if (this.isProjectLimitReached) {
			return this.projectLimitReachedMessage;
		}

		if (this.canShowCloudProjectLimitUpgrade) {
			return 'Available on Business';
		}

		return '';
	}

	/** Message for the empty state only (improved, billing-aware copy). */
	getProjectLimitMessageForEmptyState(): string {
		if (this.isProjectLimitReached) {
			return this.projectLimitReachedMessage;
		}

		if (this.canShowCloudProjectLimitUpgrade) {
			return 'Upgrade your plan to create more projects';
		}

		return '';
	}

	get shouldBlockProjectCreation(): boolean {
		return this.isDisabled || this.isProjectLimitReached || this.canShowCloudProjectLimitUpgrade;
	}

	private get isProjectLimitReached(): boolean {
		return this.licenseService.isLimitAvailable('project_limit') && this.licenseService.isLimitReached('project_limit');
	}

	private get canShowCloudProjectLimitUpgrade(): boolean {
		return this.canOpenBillingForProjectLimit &&
			!this.licenseService.hasLicense('project_limit') &&
			!this.licenseService.isLimitAvailable('project_limit');
	}

	private get projectLimitReachedMessage(): string {
		return this.licenseService.limitReachedMessage('project_limit');
	}

	get canOpenBillingForProjectLimit(): boolean {
		return this.billingStrategy === 'cloud' || this.billingStrategy === 'licensed_self_hosted';
	}

	get disabledOrganisationMessage(): string {
		return this.orgState.disabledOrganisationMessage(this.billingStrategy);
	}

	get canStartCloudTrial(): boolean {
		return canBlockProjectsWithCloudTrial({
			isDisabled: this.isDisabled,
			canStartCloud: this.billingStrategy === 'cloud' &&
				this.canShowCloudProjectLimitUpgrade &&
				this.trialEligible
		});
	}

	get canShowSelfHostedTrialUpsell(): boolean {
		return canShowSelfHostedTrialUpsell({
			isDisabled: this.isDisabled,
			billingStrategy: this.billingStrategy,
			billingConfigLoaded: this.billingConfigLoaded,
			selfHostedConfig: this.selfHostedBillingConfig
		});
	}

	get canStartSelfHostedTrial(): boolean {
		return evaluateSelfHostedTrialEligibility({
			billingStrategy: this.billingStrategy,
			billingConfigLoaded: this.billingConfigLoaded,
			selfHostedConfig: this.selfHostedBillingConfig
		});
	}

	get selfHostedTrialOffer(): TrialOffer | null {
		return resolveSelfHostedTrialOffer(this.selfHostedBillingConfig);
	}

	get trialModalMode(): 'cloud' | 'self_hosted' {
		return resolveTrialModalMode(this.canStartSelfHostedTrial);
	}

	get canStartTrial(): boolean {
		return canOfferProjectTrial({
			isDisabled: this.isDisabled,
			canStartSelfHosted: this.canShowSelfHostedTrialUpsell,
			canStartCloud: this.canStartCloudTrial
		});
	}

	get trialIntro(): string {
		return formatTrialIntro(this.trialModalMode, this.cloudTrialOffer, this.selfHostedTrialOffer);
	}

	get selfHostedTrialUpsellLead(): string {
		return formatSelfHostedTrialUpsellLead(this.selfHostedTrialOffer);
	}

	get cloudTrialUpsellLead(): string {
		return formatCloudTrialUpsellLead(this.cloudTrialOffer);
	}

	get emptyStateCreateDisabled(): boolean {
		return this.shouldBlockProjectCreation;
	}

	// Open the trial modal (terms/limits, no card collection). The modal owns
	// the trial POST; onTrialStarted runs the provisioning poll afterwards.
	openTrialModal() {
		if (!this.canStartTrial || this.isStartingTrial) return;
		this.trialModal?.open();
	}

	// "or subscribe now" from the trial modal: skip the trial and go straight to
	// the paid checkout on the billing page (plans view lives there). No trial
	// is consumed.
	onSubscribeInsteadOfTrial() {
		this.billingPaymentDetailsService.navigateToBillingWithManagePlan(this.router);
	}

	// Runs after the modal reports the trial started (with or without a card).
	// Enters the provisioning state and polls for the entitlement webhook so the
	// empty state unlocks itself instead of re-offering "Start trial".
	async onTrialStarted() {
		this.isStartingTrial = true;
		this.generalService.showNotification({
			message: 'Your free trial has started.',
			style: 'success'
		});
		// Refresh eligibility now so the org is marked ineligible immediately (it
		// just consumed its one trial), keeping the nav pill and CTA in sync.
		this.trialProvisioning = true;
		void this.trialStatusService.refresh();
		// isStartingTrial/trialProvisioning are cleared inside the poll (unlock,
		// timeout, or destroy).
		await this.pollForTrialEntitlement();
	}

	// Manual unlock retry for the rare case the entitlement webhook is slower than
	// the poll window. Re-enters the polling state instead of forcing a full reload.
	retryTrialUnlock() {
		if (this.isStartingTrial) return;
		this.isStartingTrial = true;
		this.trialProvisioning = true;
		void this.pollForTrialEntitlement();
	}

	// Bounded poll: refresh the license cache until project creation unlocks, then
	// reload projects so "Create a Project" enables. The unlock signal is
	// shouldBlockProjectCreation (the actual gate), not canStartTrial, since
	// eligibility flips to false the moment the trial starts and would otherwise
	// end the poll before the entitlement lands. A token guards against stale
	// timers after destroy; on timeout we stay in the provisioning state (with a
	// Refresh action) rather than re-offering "Start trial".
	private async pollForTrialEntitlement() {
		const token = ++this.trialPollToken;

		const unlocked = await pollUntil({
			budgetMs: POLL_BUDGET_MS,
			request: async () => {
				await Promise.all([
					this.licenseService.loadAllLicenses(),
					this.loadBillingContext()
				]);
				return this.shouldBlockProjectCreation;
			},
			isDone: (blocked) => !blocked
		});

		if (token !== this.trialPollToken) return;

		if (unlocked) {
			await this.getProjects();
			if (token !== this.trialPollToken) return;
			void this.trialStatusService.refresh();
			this.isStartingTrial = false;
			this.trialProvisioning = false;
			return;
		}

		this.isStartingTrial = false;
		this.generalService.showNotification({
			message: 'Trial started and is activating. Use Refresh if projects stay locked.',
			style: 'info'
		});
	}

	ngOnDestroy() {
		this.trialPollToken++;
		this.trialEligibleSub?.unsubscribe();
		this.trialOfferSub?.unsubscribe();
	}
}
