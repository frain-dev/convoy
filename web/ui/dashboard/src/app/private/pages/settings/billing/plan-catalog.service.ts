import {Injectable} from '@angular/core';
import {Plan} from './plan.service';

// Plan classification. The pure classifiers use backend-guaranteed signals:
// product_type (Overwatch plans.product_type is not-null: cloud | self_hosted)
// and the canonical enterprise keys below. The default-to-overwatch merge
// (findDefaultPlanComparison / resolvePlanForApi / shouldContactForMissingCloudPlan)
// stays name-keyed by design: it mirrors the backend merge in
// api/handlers/billing_plans.go (mergePlansWithFeatures), which joins config and
// service plans by lowercased name.
const PRODUCT_TYPE_SELF_HOSTED = 'self_hosted';
// Canonical enterprise plan keys. Backend (Overwatch plans.key) uses the
// cloud_/self_hosted_ forms; the bundled default comparison plan uses 'enterprise'.
const ENTERPRISE_KEYS = ['enterprise', 'cloud_enterprise', 'self_hosted_enterprise'];
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
		return plan.product_type === PRODUCT_TYPE_SELF_HOSTED;
	}

	isEnterprisePlan(plan: Plan): boolean {
		const key = (plan.key || plan.id || '').toLowerCase();
		return ENTERPRISE_KEYS.includes(key);
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
