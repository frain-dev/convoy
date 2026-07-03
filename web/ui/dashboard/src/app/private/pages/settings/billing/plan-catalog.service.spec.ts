import {Plan} from './plan.service';
import {PlanCatalogService} from './plan-catalog.service';

describe('PlanCatalogService', () => {
	let service: PlanCatalogService;

	const defaultPlans: Plan[] = [
		plan({
			id: 'default-pro',
			name: 'Pro',
			description: 'Default cloud Pro',
			features: [{ name: 'Retries', category: 'core', value: 'Supported' }]
		}),
		plan({
			id: 'default-enterprise',
			key: 'enterprise',
			name: 'Enterprise',
			description: 'Default cloud Enterprise',
			checkout_enabled: false,
			requires_contact: true,
			features: [{ name: 'SAML', category: 'security', value: 'Supported' }]
		})
	];

	beforeEach(() => {
		service = new PlanCatalogService();
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

		expect(enterprise?.id).toBe('default-enterprise');
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
				name: 'Pro',
				product_type: 'cloud'
			}),
			plan({
				id: 'self-hosted-premium',
				key: 'self_hosted_premium',
				name: 'Premium',
				product_type: 'self_hosted'
			})
		], defaultPlans, true);

		expect(catalog.plans.map(plan => plan.id)).toEqual(['self-hosted-premium']);
		expect(catalog.billingPlans.map(plan => plan.id)).toEqual(['self-hosted-premium']);
		expect(catalog.plansUnavailableMessage).toBe('');
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
