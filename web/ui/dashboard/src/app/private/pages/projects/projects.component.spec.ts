import { ComponentFixture, TestBed } from '@angular/core/testing';
import { CUSTOM_ELEMENTS_SCHEMA } from '@angular/core';
import { RouterTestingModule } from '@angular/router/testing';
import { of, Subject } from 'rxjs';
import { ProjectsComponent } from './projects.component';
import { PrivateService } from '../../private.service';
import { LicensesService } from 'src/app/services/licenses/licenses.service';
import { OrganisationStateService } from 'src/app/services/organisation-state/organisation-state.service';
import { GeneralService } from 'src/app/services/general/general.service';
import { TrialStatusService } from 'src/app/services/trial-status/trial-status.service';
import { BillingPaymentDetailsService } from '../settings/billing/billing-payment-details.service';

describe('ProjectsComponent trial CTA', () => {
	let component: ProjectsComponent;
	let fixture: ComponentFixture<ProjectsComponent>;

	beforeEach(async () => {
		await TestBed.configureTestingModule({
			imports: [RouterTestingModule],
			declarations: [ProjectsComponent],
			schemas: [CUSTOM_ELEMENTS_SCHEMA],
			providers: [
				{
					provide: PrivateService,
					useValue: {
						projects$: new Subject(),
						getProjects: () => Promise.resolve({ data: [] })
					}
				},
				{
					provide: LicensesService,
					useValue: {
						loadAllLicenses: () => Promise.resolve(),
						hasLicense: () => false,
						isLimitAvailable: () => false,
						isLimitReached: () => false,
						limitReachedMessage: () => ''
					}
				},
				{
					provide: OrganisationStateService,
					useValue: {
						isDisabled: () => false,
						disabledOrganisationMessage: () => ''
					}
				},
				{ provide: GeneralService, useValue: { showNotification: () => {} } },
				{
					provide: TrialStatusService,
					useValue: {
						eligible$: of(false),
						offer$: of(null),
						refresh: () => Promise.resolve()
					}
				},
				{
					provide: BillingPaymentDetailsService,
					useValue: {
						getBillingConfig: () =>
							of({
								data: {
									strategy: 'oss',
									self_hosted: {
										license_configured: false,
										trial_offer: {
											duration_days: 14,
											plan_name: 'Self-Hosted Premium',
											limits: [
												{ key: 'project_limit', label: 'Projects', value: 2 },
												{ key: 'org_limit', label: 'Organizations', value: 1 },
												{ key: 'user_limit', label: 'Team members', value: 1 }
											]
										}
									}
								}
							})
					}
				}
			]
		}).compileComponents();

		fixture = TestBed.createComponent(ProjectsComponent);
		component = fixture.componentInstance;
	});

	it('offers self-hosted trial upsell without blocking OSS project create', async () => {
		await component.ngOnInit();
		expect(component.canStartSelfHostedTrial).toBeTrue();
		expect(component.canShowSelfHostedTrialUpsell).toBeTrue();
		expect(component.canStartTrial).toBeTrue();
		expect(component.shouldBlockProjectCreation).toBeFalse();
		expect(component.canStartCloudTrial).toBeFalse();
		expect(component.trialModalMode).toBe('self_hosted');
	});

	it('blocks cloud project create until trial starts', async () => {
		const billing = TestBed.inject(BillingPaymentDetailsService) as { getBillingConfig: () => unknown };
		billing.getBillingConfig = () =>
			of({
				data: {
					strategy: 'cloud',
					self_hosted: null
				}
			});
		const trialStatus = TestBed.inject(TrialStatusService) as { eligible$: unknown; refresh: () => Promise<void> };
		trialStatus.eligible$ = of(true);

		await component.ngOnInit();
		expect(component.canStartCloudTrial).toBeTrue();
		expect(component.shouldBlockProjectCreation).toBeTrue();
		expect(component.emptyStateCreateDisabled).toBeTrue();
		expect(component.canShowSelfHostedTrialUpsell).toBeFalse();
	});

	it('does not offer a cloud trial on OSS', async () => {
		component.billingStrategy = 'oss';
		component.billingConfigLoaded = true;
		component.selfHostedBillingConfig = { license_configured: true };
		component.trialEligible = true;
		expect(component.canStartSelfHostedTrial).toBeFalse();
		expect(component.canStartTrial).toBeFalse();
	});
});
