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
		expect(catalog.overwatchPlans).toEqual([]);
	});

	it('merges cloud API plans into matching default cards in cloud mode', () => {
		const catalog = service.buildCatalog([
			plan({
				id: 'cloud-pro',
				key: 'cloud_pro',
				name: 'Pro',
				product_type: 'cloud',
				description: 'Cloud Pro from Overwatch',
				price: 199,
				features: []
			})
		], defaultPlans, false);

		const pro = catalog.plans.find(plan => plan.name === 'Pro');

		expect(pro?.id).toBe('cloud-pro');
		expect(pro?.description).toBe('Cloud Pro from Overwatch');
		expect(pro?.price).toBe(199);
		expect(pro?.features).toEqual(defaultPlans[0].features);
		expect(catalog.overwatchPlans.map(plan => plan.id)).toEqual(['cloud-pro']);
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
		expect(catalog.overwatchPlans.map(plan => plan.id)).toEqual(['self-hosted-premium']);
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
