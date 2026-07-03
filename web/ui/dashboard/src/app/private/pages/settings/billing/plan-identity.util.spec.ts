import {Plan} from './plan.service';
import {isEnterprisePlanKey, PLAN_KEYS, plansMatch, resolvePlanKey} from './plan-identity.util';

describe('plan-identity.util', () => {
	it('prefers billing plan.key when present', () => {
		expect(resolvePlanKey({ key: 'self_hosted_premium', id: 'uuid', name: 'Self-Hosted Premium' }))
			.toBe(PLAN_KEYS.SELF_HOSTED_PREMIUM);
	});

	it('aliases the served business SKU onto the premium presentation key', () => {
		expect(resolvePlanKey({ key: 'business_annual', id: 'uuid', name: 'Business - Annual' }))
			.toBe(PLAN_KEYS.SELF_HOSTED_PREMIUM);
	});

	it('aliases the served enterprise SKU onto the enterprise presentation key and gates it', () => {
		const plan = { key: 'enterprise', id: 'uuid', name: 'Enterprise' };
		expect(resolvePlanKey(plan)).toBe(PLAN_KEYS.SELF_HOSTED_ENTERPRISE);
		expect(isEnterprisePlanKey(plan)).toBeTrue();
	});

	it('maps legacy dashboard default ids for older embedded UI cards', () => {
		expect(resolvePlanKey({ id: 'pro', name: 'Pro' })).toBe(PLAN_KEYS.CLOUD_PRO);
		expect(resolvePlanKey({ id: 'enterprise', name: 'Enterprise' })).toBe(PLAN_KEYS.CLOUD_ENTERPRISE);
	});

	it('maps exact legacy display names when billing omits plan.key', () => {
		expect(resolvePlanKey({ id: 'uuid', name: 'Self-Hosted Premium' })).toBe(PLAN_KEYS.SELF_HOSTED_PREMIUM);
		expect(resolvePlanKey({ id: 'uuid', name: 'Cloud Pro' })).toBe(PLAN_KEYS.CLOUD_PRO);
	});

	it('matches plans by key before id or exact name', () => {
		const apiPlan: Plan = {
			id: '6a1b6ab7',
			key: 'self_hosted_premium',
			name: 'Self-Hosted Premium',
			description: '',
			price: 0,
			currency: 'USD',
			interval: 'month',
			features: []
		};
		const defaultPlan: Plan = {
			id: 'self_hosted_premium',
			key: 'self_hosted_premium',
			name: 'Self-Hosted Premium',
			description: '',
			price: 0,
			currency: 'USD',
			interval: 'month',
			features: [{ name: 'Retries', category: 'core', value: 'Supported' }]
		};

		expect(plansMatch(apiPlan, defaultPlan)).toBeTrue();
	});
});
