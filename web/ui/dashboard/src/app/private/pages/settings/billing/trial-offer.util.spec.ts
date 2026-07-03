import {
	canBlockProjectsWithCloudTrial,
	canShowSelfHostedTrialUpsell,
	canStartSelfHostedTrial,
	filterPlansToTrialPlan,
	formatCloudTrialUpsellLead,
	formatSelfHostedTrialUpsellLead,
	formatTrialDuration,
	formatTrialIntro,
	formatTrialLimitLine,
	hasActiveSelfHostedCheckout,
	hasTrialLimits,
	resolveSelfHostedTrialOffer,
	resolveTrialModalMode,
	resolveTrialOffer,
	resolveTrialPlanName,
	trialFeaturesLead,
	trialFeaturesLine,
	TrialOffer
} from './trial-offer.util';

describe('trial-offer.util', () => {
	const cloudPlan = { id: '1', key: 'cloud_pro', name: 'Cloud Pro', description: '', price: 0, currency: 'USD', interval: 'month', features: [] };
	const shPlan = { id: '2', key: 'self_hosted_premium', name: 'Self-Hosted Premium', description: '', price: 0, currency: 'USD', interval: 'month', features: [] };

	it('singularizes countable trial limits at 1', () => {
		expect(formatTrialLimitLine({ key: 'project_limit', label: 'Projects', value: 1 })).toBe('1 Project');
		expect(formatTrialLimitLine({ key: 'org_limit', label: 'Organizations', value: 1 })).toBe('1 Organization');
		expect(formatTrialLimitLine({ key: 'user_limit', label: 'Team members', value: 1 })).toBe('1 Team member');
	});

	it('formats hour-based trial durations from billing', () => {
		const offer: TrialOffer = { duration_count: 1, duration_unit: 'hour', plan_name: 'Cloud Pro' };
		expect(formatTrialDuration(offer)).toBe('1 hour');
		expect(formatTrialIntro('cloud', offer, null)).toBe('Try Cloud Pro free for 1 hour. No payment method required.');
	});

	it('formats day-based trial durations from billing', () => {
		const offer: TrialOffer = { duration_count: 14, duration_unit: 'day', duration_days: 14, plan_name: 'Cloud Pro' };
		expect(formatTrialDuration(offer)).toBe('14 days');
	});

	it('prefers billing plan_name over catalog fallbacks', () => {
		expect(resolveTrialPlanName('cloud', { plan_name: 'Cloud Pro Monthly' }, [cloudPlan])).toBe('Cloud Pro Monthly');
		expect(resolveTrialPlanName('self_hosted', { plan_name: 'Self-Hosted Premium' }, [shPlan])).toBe('Self-Hosted Premium');
	});

	it('falls back to catalog plan keys when billing omits plan_name', () => {
		expect(resolveTrialPlanName('cloud', null, [cloudPlan])).toBe('Cloud Pro');
		expect(resolveTrialPlanName('self_hosted', null, [shPlan])).toBe('Self-Hosted Premium');
	});

	it('prefers billing plan_key when present', () => {
		expect(
			resolveTrialPlanName('self_hosted', { plan_key: 'self_hosted_premium' }, [shPlan])
		).toBe('Self-Hosted Premium');
	});

	it('resolves cloud vs self-hosted trial offers from one helper', () => {
		const cloudOffer: TrialOffer = { duration_count: 1, duration_unit: 'hour', plan_name: 'Cloud Pro' };
		expect(resolveTrialOffer('cloud', cloudOffer, null)).toBe(cloudOffer);
		expect(resolveTrialOffer('self_hosted', null, null)).toEqual({ requires_card: false, limits: [] });
	});

	it('hides limit copy when self-hosted trial has no caps', () => {
		const offer: TrialOffer = { plan_name: 'Self-Hosted Premium', limits: [] };
		expect(hasTrialLimits(offer)).toBe(false);
		expect(trialFeaturesLead('Self-Hosted Premium', offer)).toBe('All supported Self-Hosted Premium');
		expect(trialFeaturesLine('Self-Hosted Premium', offer)).toBe('All supported Self-Hosted Premium features are included.');
	});

	it('uses "all other" copy when trial limits are listed separately', () => {
		const offer: TrialOffer = {
			plan_name: 'Cloud Pro',
			limits: [{ key: 'daily_event_limit', label: 'Events / day', value: 1000 }]
		};
		expect(trialFeaturesLead('Cloud Pro', offer)).toBe('All other supported Cloud Pro');
		expect(trialFeaturesLine('Cloud Pro', offer)).toBe('All other supported Cloud Pro features are included.');
	});

	it('filters the catalog to the trial SKU for the features dialog', () => {
		const shPlans = [
			{ id: '1', key: 'self_hosted_premium', name: 'Self-Hosted Premium', description: '', price: 0, currency: 'USD', interval: 'month', features: [] },
			{ id: '2', key: 'self_hosted_enterprise', name: 'Self-Hosted Enterprise', description: '', price: 0, currency: 'USD', interval: 'month', features: [] }
		];
		const cloudPlans = [
			{ id: '3', key: 'cloud_pro', name: 'Cloud Pro', description: '', price: 0, currency: 'USD', interval: 'month', features: [] },
			{ id: '4', key: 'cloud_enterprise', name: 'Cloud Enterprise', description: '', price: 0, currency: 'USD', interval: 'month', features: [] }
		];
		expect(filterPlansToTrialPlan(shPlans, 'Self-Hosted Premium')).toEqual([shPlans[0]]);
		expect(filterPlansToTrialPlan(cloudPlans, 'Cloud Pro', 'cloud_pro')).toEqual([cloudPlans[0]]);
	});

	it('formats a short OSS projects upsell lead without duration copy', () => {
		expect(formatSelfHostedTrialUpsellLead({ plan_name: 'Self-Hosted Premium' })).toBe(
			'Want Self-Hosted Premium features?'
		);
	});

	it('formats a short cloud projects upsell lead without duration copy', () => {
		expect(formatCloudTrialUpsellLead({ plan_name: 'Cloud Pro' })).toBe('Want Cloud Pro features?');
	});

	it('gates self-hosted trial eligibility consistently for billing and projects', () => {
		const config = { license_configured: false, trial_offer: { duration_days: 14 } };
		const eligibility = {
			billingStrategy: 'oss',
			billingConfigLoaded: true,
			selfHostedConfig: config
		};
		expect(canStartSelfHostedTrial(eligibility)).toBeTrue();
		expect(canStartSelfHostedTrial({
			...eligibility,
			selfHostedConfig: { ...config, license_configured: true }
		})).toBeFalse();
		expect(canShowSelfHostedTrialUpsell(eligibility)).toBeTrue();
		expect(canShowSelfHostedTrialUpsell({ ...eligibility, isDisabled: true })).toBeFalse();
		expect(canBlockProjectsWithCloudTrial({ isDisabled: false, canStartCloud: true })).toBeTrue();
		expect(canBlockProjectsWithCloudTrial({ isDisabled: false, canStartCloud: false })).toBeFalse();
		expect(hasActiveSelfHostedCheckout({ active_checkout: { attempt_id: 'chk_1' } })).toBeTrue();
		expect(resolveSelfHostedTrialOffer(config)).toEqual(config.trial_offer);
		expect(resolveTrialModalMode(true)).toBe('self_hosted');
		expect(resolveTrialModalMode(false)).toBe('cloud');
	});
});
