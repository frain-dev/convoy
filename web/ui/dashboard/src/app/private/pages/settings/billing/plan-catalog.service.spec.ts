import {Plan} from './plan.service';
import {PlanCatalogService} from './plan-catalog.service';
import {PlanService} from './plan.service';

describe('PlanCatalogService', () => {
	let service: PlanCatalogService;
	let planService: PlanService;

	const defaultPlans: Plan[] = [
		plan({
			id: 'cloud_pro',
			key: 'cloud_pro',
			name: 'Pro',
			description: 'Default cloud Pro',
			features: [{ name: 'Retries', category: 'core', value: 'Supported' }]
		}),
		plan({
			id: 'cloud_enterprise',
			key: 'cloud_enterprise',
			name: 'Enterprise',
			description: 'Default cloud Enterprise',
			checkout_enabled: false,
			requires_contact: true,
			features: [{ name: 'SAML', category: 'security', value: 'Supported' }]
		})
	];

	beforeEach(() => {
		service = new PlanCatalogService();
		planService = new PlanService({} as any);
	});

	it('ignores self-hosted API plans with matching cloud default names in cloud mode', () => {
		const catalog = service.buildCatalog([
			plan({
				id: 'self-hosted-enterprise',
				key: 'self_hosted_enterprise',
				name: 'Enterprise',
				product_type: 'self_hosted',
				description: 'Legacy self-hosted Enterprise',
				checkout_enabled: true,
				requires_contact: false
			})
		], defaultPlans, false);

		const enterprise = catalog.plans.find(plan => plan.name === 'Enterprise');

		expect(enterprise?.id).toBe('cloud_enterprise');
		expect(enterprise?.requires_contact).toBeTrue();
		expect(enterprise?.checkout_enabled).toBeFalse();
		expect(catalog.billingPlans).toEqual([]);
	});

	it('merges cloud API plans into matching default cards in cloud mode', () => {
		const catalog = service.buildCatalog([
			plan({
				id: 'cloud-pro',
				key: 'cloud_pro',
				name: 'Pro',
				product_type: 'cloud',
				description: 'Cloud Pro from billing service',
				price: 199,
				features: []
			})
		], defaultPlans, false);

		const pro = catalog.plans.find(plan => plan.id === 'cloud-pro');

		expect(pro?.name).toBe('Pro');
		expect(pro?.description).toBe('Cloud Pro from billing service');
		expect(pro?.price).toBe(199);
		expect(pro?.features).toEqual(defaultPlans[0].features);
		expect(catalog.billingPlans.map(plan => plan.id)).toEqual(['cloud-pro']);
	});

	it('uses billing plan names when API names differ from default comparison labels', () => {
		const catalog = service.buildCatalog([
			plan({
				id: 'cloud-pro',
				key: 'cloud_pro',
				name: 'Cloud Pro',
				product_type: 'cloud',
				description: 'Professional cloud plan',
				price: 99,
				features: []
			}),
			plan({
				id: 'cloud-enterprise',
				key: 'cloud_enterprise',
				name: 'Cloud Enterprise',
				product_type: 'cloud',
				description: 'Enterprise cloud plan',
				checkout_enabled: false,
				requires_contact: true,
				features: []
			})
		], defaultPlans, false);

		expect(catalog.plans.map(plan => plan.name)).toEqual(['Cloud Pro', 'Cloud Enterprise']);
		expect(catalog.plans[0].features).toEqual(defaultPlans[0].features);
		expect(catalog.plans[1].features).toEqual(defaultPlans[1].features);
	});

	it('returns self-hosted plans in self-hosted billing mode', () => {
		const catalog = service.buildCatalog([
			plan({
				id: 'cloud-pro',
				key: 'cloud_pro',
				name: 'Pro',
				product_type: 'cloud'
			}),
			plan({
				id: 'self-hosted-premium',
				key: 'self_hosted_premium',
				name: 'Self-Hosted Premium',
				product_type: 'self_hosted'
			})
		], defaultPlans, true);

		expect(catalog.plans.map(plan => plan.id)).toEqual(['self-hosted-premium']);
		expect(catalog.billingPlans.map(plan => plan.id)).toEqual(['self-hosted-premium']);
		expect(catalog.plansUnavailableMessage).toBe('');
	});

	it('merges self-hosted premium features from defaults using billing plan.key', () => {
		const shDefaults = planService.getDefaultSelfHostedPlanComparison().plans;
		const catalog = service.buildCatalog([
			plan({
				id: '6a1b6ab7-5ea6-43ff-88ba-1128c8f6b02c',
				key: 'self_hosted_premium',
				name: 'Self-Hosted Premium',
				product_type: 'self_hosted',
				features: []
			}),
			plan({
				id: '3984374d-bdac-4796-b450-ee3ba0439b43',
				key: 'self_hosted_enterprise',
				name: 'Self-Hosted Enterprise',
				product_type: 'self_hosted',
				checkout_enabled: false,
				requires_contact: true,
				features: []
			})
		], shDefaults, true);

		const premium = catalog.plans.find(plan => plan.key === 'self_hosted_premium');
		const enterprise = catalog.plans.find(plan => plan.key === 'self_hosted_enterprise');

		expect(premium?.features.length).toBeGreaterThan(0);
		expect(enterprise?.features.length).toBeGreaterThan(0);
		expect(service.resolvePlanForApi(premium as Plan, catalog.billingPlans).planExistsInCatalog).toBeTrue();
	});

	it('matches legacy cloud default cards without billing plan.key', () => {
		const legacyDefaults: Plan[] = [
			plan({ id: 'pro', name: 'Pro', features: defaultPlans[0].features }),
			plan({
				id: 'enterprise',
				key: 'enterprise',
				name: 'Enterprise',
				checkout_enabled: false,
				requires_contact: true,
				features: defaultPlans[1].features
			})
		];

		const catalog = service.buildCatalog([
			plan({
				id: 'cloud-pro',
				key: 'cloud_pro',
				name: 'Cloud Pro',
				product_type: 'cloud',
				features: []
			})
		], legacyDefaults, false);

		expect(catalog.plans[0].features).toEqual(defaultPlans[0].features);
	});

	it('keeps Enterprise contact-only when no cloud Enterprise API plan is eligible', () => {
		const catalog = service.buildCatalog([
			plan({
				id: 'self-hosted-enterprise',
				key: 'self_hosted_enterprise',
				name: 'Enterprise',
				product_type: 'self_hosted',
				requires_contact: true,
				checkout_enabled: false
			})
		], defaultPlans, false);

		const enterprise = catalog.plans.find(plan => plan.name === 'Enterprise');

		expect(enterprise?.requires_contact).toBeTrue();
		expect(enterprise?.checkout_enabled).toBeFalse();
		expect(service.shouldContactForMissingCloudPlan(enterprise as Plan, false, false)).toBeTrue();
	});
});

function plan(overrides: Partial<Plan>): Plan {
	return {
		id: 'plan-id',
		name: 'Plan',
		description: 'Plan description',
		price: 0,
		currency: 'USD',
		interval: 'month',
		features: [],
		...overrides
	};
}
