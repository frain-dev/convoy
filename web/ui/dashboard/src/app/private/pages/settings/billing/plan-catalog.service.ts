import {Injectable} from '@angular/core';
import {Plan} from './plan.service';

// Plan classification uses product_type and canonical enterprise keys.
const PRODUCT_TYPE_SELF_HOSTED = 'self_hosted';
const PRODUCT_TYPE_CLOUD = 'cloud';
// Canonical enterprise plan keys.
const ENTERPRISE_KEYS = ['enterprise', 'cloud_enterprise', 'self_hosted_enterprise'];
const ENTERPRISE_TOKEN = 'enterprise';
const PRO_TOKENS = ['premium', 'pro'];

type PlanTier = 'enterprise' | 'pro' | 'exact';

const CADENCE_MONTHLY = 'monthly';
const CADENCE_ANNUAL = 'annual';

export interface PlanCatalog {
	plans: Plan[];
	billingPlans: Plan[];
	plansUnavailableMessage: string;
}

export interface ResolvedPlanForApi {
	planExistsInCatalog: boolean;
	planIdForApi: string;
}

@Injectable({ providedIn: 'root' })
export class PlanCatalogService {
	isSelfHostedPlan(plan: Plan): boolean {
		return plan.product_type === PRODUCT_TYPE_SELF_HOSTED;
	}

	isCloudPlan(plan: Plan): boolean {
		return plan.product_type === PRODUCT_TYPE_CLOUD;
	}

	isEnterprisePlan(plan: Plan): boolean {
		const key = (plan.key || plan.id || '').toLowerCase();
		return ENTERPRISE_KEYS.includes(key);
	}

	shouldContactForMissingCloudPlan(plan: Plan, isSelfHostedBilling: boolean, planExistsInCatalog: boolean): boolean {
		const name = plan.name.toLowerCase();
		return !isSelfHostedBilling && (name.includes('pro') || name.includes(ENTERPRISE_TOKEN)) && !planExistsInCatalog;
	}

	resolveCheckoutCadence(plan: Plan): string {
		const pricingInterval = plan.pricing_options?.find(option => !!option?.interval)?.interval;
		const intervalsInterval = plan.intervals?.find(interval => !!interval);

		return this.normalizeCheckoutCadence(pricingInterval) ||
			this.normalizeCheckoutCadence(intervalsInterval) ||
			this.normalizeCheckoutCadence(plan.interval) ||
			CADENCE_MONTHLY;
	}

	normalizeCheckoutCadence(value?: string | null): string {
		const raw = (value || '').trim().toLowerCase();
		if (!raw) return '';
		if (raw === 'month') return CADENCE_MONTHLY;
		if (raw === 'year' || raw === 'yearly') return CADENCE_ANNUAL;
		return raw;
	}

	mergePlanWithDefaultComparison(plan: Plan, defaultPlans: Plan[]): Plan {
		const defaultPlan = this.findDefaultPlanComparison(plan, defaultPlans);
		if (!defaultPlan || (plan.features && plan.features.length > 0)) {
			return plan;
		}

		return {
			...plan,
			features: defaultPlan.features,
			description: plan.description || defaultPlan.description,
			price: plan.price || defaultPlan.price,
			currency: plan.currency || defaultPlan.currency,
			interval: plan.interval || defaultPlan.interval
		};
	}

	// Shared tier matching keeps cloud default-card merge and billing-service feature
	// merge aligned. Cloud lists default comparison cards then overlays API plans;
	// self-hosted lists API plans directly but uses the same tier rules for features.
	private planTier(plan: Plan): PlanTier {
		if (this.isEnterprisePlan(plan) || plan.name.toLowerCase().includes(ENTERPRISE_TOKEN)) {
			return 'enterprise';
		}
		const key = (plan.key || plan.id || '').toLowerCase();
		const name = plan.name.toLowerCase();
		if (PRO_TOKENS.some(token => name.includes(token) || key.includes(token))) {
			return 'pro';
		}
		return 'exact';
	}

	findDefaultPlanComparison(plan: Plan, defaultPlans: Plan[]): Plan | undefined {
		const tier = this.planTier(plan);
		if (tier === 'enterprise') {
			return defaultPlans.find(defaultPlan => defaultPlan.name.toLowerCase().includes(ENTERPRISE_TOKEN));
		}
		if (tier === 'pro') {
			return defaultPlans.find(defaultPlan => defaultPlan.name.toLowerCase().includes('pro'));
		}
		return defaultPlans.find(defaultPlan => defaultPlan.name.toLowerCase() === plan.name.toLowerCase());
	}

	findBillingPlanForDefault(defaultPlan: Plan, billingPlans: Plan[]): Plan | undefined {
		const tier = this.planTier(defaultPlan);
		if (tier === 'enterprise') {
			return billingPlans.find(plan => this.planTier(plan) === 'enterprise');
		}
		if (tier === 'pro') {
			return billingPlans.find(plan => this.planTier(plan) === 'pro');
		}
		return billingPlans.find(plan => plan.name.toLowerCase() === defaultPlan.name.toLowerCase());
	}

	resolvePlanForApi(selectedPlanData: Plan, billingPlans: Plan[]): ResolvedPlanForApi {
		const planLower = selectedPlanData.name.toLowerCase();
		const billingPlan = billingPlans.find(p => {
			const pNameLower = p.name.toLowerCase();
			return (planLower.includes(pNameLower) || pNameLower.includes(planLower)) || p.id === selectedPlanData.id;
		});

		return {
			planExistsInCatalog: !!billingPlan,
			planIdForApi: billingPlan?.id ?? selectedPlanData.id
		};
	}

	// Build the displayed plan catalog from the API response and default comparison data.
	buildCatalog(plansFromApi: Plan[], defaultPlans: Plan[], isSelfHostedBilling: boolean): PlanCatalog {
		if (plansFromApi.length === 0) {
			return {
				plans: [],
				billingPlans: [],
				plansUnavailableMessage: 'Plans are not available right now. Please try again later.'
			};
		}

		const billingPlans = plansFromApi.filter((plan: Plan) => isSelfHostedBilling ? this.isSelfHostedPlan(plan) : this.isCloudPlan(plan));
		let plans: Plan[];
		let plansUnavailableMessage = '';

		if (isSelfHostedBilling) {
			plans = billingPlans.map((plan: Plan) => this.mergePlanWithDefaultComparison(plan, defaultPlans));
			if (plans.length === 0) {
				plansUnavailableMessage = 'Self-hosted plans are not available right now. Please try again later.';
			}
		} else {
			plans = defaultPlans.map((defaultPlan: Plan) => {
				const billingPlan = this.findBillingPlanForDefault(defaultPlan, billingPlans);

				if (billingPlan) {
					return this.mergePlanWithDefaultComparison(billingPlan, [defaultPlan]);
				}

				return defaultPlan;
			});
		}

		return { plans, billingPlans, plansUnavailableMessage };
	}
}
