import {Injectable} from '@angular/core';
import {Plan} from './plan.service';

// Plan classification tokens. The data model does not carry a single stable
// catalog key shared with the backend, so classification still matches on
// product_type plus plan-name substrings; these constants only name the
// literals that were previously inlined and do not change classification.
const PRODUCT_TYPE_SELF_HOSTED = 'self_hosted';
const SELF_HOSTED_NAME_TOKENS = ['self-hosted', 'self hosted'];
const ENTERPRISE_TOKEN = 'enterprise';
const PRO_TOKENS = ['premium', 'pro'];

const CADENCE_MONTHLY = 'monthly';
const CADENCE_ANNUAL = 'annual';

export interface PlanCatalog {
	plans: Plan[];
	overwatchPlans: Plan[];
	plansUnavailableMessage: string;
}

export interface ResolvedPlanForApi {
	planExistsInOverwatch: boolean;
	planIdForApi: string;
}

@Injectable({ providedIn: 'root' })
export class PlanCatalogService {
	isSelfHostedPlan(plan: Plan): boolean {
		const name = plan.name.toLowerCase();
		return plan.product_type === PRODUCT_TYPE_SELF_HOSTED || SELF_HOSTED_NAME_TOKENS.some(token => name.includes(token));
	}

	isEnterprisePlan(plan: Plan): boolean {
		const key = (plan.key || plan.id || '').toLowerCase();
		const name = plan.name.toLowerCase();
		return key.includes(ENTERPRISE_TOKEN) || name.includes(ENTERPRISE_TOKEN);
	}

	shouldContactForMissingCloudPlan(plan: Plan, isSelfHostedBilling: boolean, planExistsInOverwatch: boolean): boolean {
		const name = plan.name.toLowerCase();
		return !isSelfHostedBilling && (name.includes('pro') || name.includes(ENTERPRISE_TOKEN)) && !planExistsInOverwatch;
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

	findDefaultPlanComparison(plan: Plan, defaultPlans: Plan[]): Plan | undefined {
		const name = plan.name.toLowerCase();
		if (name.includes(ENTERPRISE_TOKEN)) {
			return defaultPlans.find(defaultPlan => defaultPlan.name.toLowerCase().includes(ENTERPRISE_TOKEN));
		}
		if (PRO_TOKENS.some(token => name.includes(token))) {
			return defaultPlans.find(defaultPlan => defaultPlan.name.toLowerCase().includes('pro'));
		}
		return defaultPlans.find(defaultPlan => defaultPlan.name.toLowerCase() === name);
	}

	resolvePlanForApi(selectedPlanData: Plan, overwatchPlans: Plan[]): ResolvedPlanForApi {
		const planLower = selectedPlanData.name.toLowerCase();
		const overwatchPlan = overwatchPlans.find(p => {
			const pNameLower = p.name.toLowerCase();
			return (planLower.includes(pNameLower) || pNameLower.includes(planLower)) || p.id === selectedPlanData.id;
		});

		return {
			planExistsInOverwatch: !!overwatchPlan,
			planIdForApi: overwatchPlan?.id ?? selectedPlanData.id
		};
	}

	// Build the displayed plan catalog from the API response and the default
	// comparison data. Pure transform; the component owns loading flags and
	// selection reconciliation.
	buildCatalog(plansFromApi: Plan[], defaultPlans: Plan[], isSelfHostedBilling: boolean): PlanCatalog {
		if (plansFromApi.length === 0) {
			return {
				plans: [],
				overwatchPlans: [],
				plansUnavailableMessage: 'Plans are not available right now. Please try again later.'
			};
		}

		const overwatchPlans = plansFromApi;
		let plans: Plan[];
		let plansUnavailableMessage = '';

		if (isSelfHostedBilling) {
			const selfHostedPlans = plansFromApi.filter((plan: Plan) => this.isSelfHostedPlan(plan));
			plans = selfHostedPlans.map((plan: Plan) => this.mergePlanWithDefaultComparison(plan, defaultPlans));
			if (plans.length === 0) {
				plansUnavailableMessage = 'Self-hosted plans are not available right now. Please try again later.';
			}
		} else {
			const overwatchPlansMap = new Map<string, Plan>();
			plansFromApi.forEach((plan: Plan) => {
				overwatchPlansMap.set(plan.name.toLowerCase(), plan);
			});

			plans = defaultPlans.map((defaultPlan: Plan) => {
				const overwatchPlan = overwatchPlansMap.get(defaultPlan.name.toLowerCase());

				if (overwatchPlan) {
					return this.mergePlanWithDefaultComparison(overwatchPlan, [defaultPlan]);
				}

				return defaultPlan;
			});
		}

		return { plans, overwatchPlans, plansUnavailableMessage };
	}
}
