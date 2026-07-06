import {Injectable} from '@angular/core';
import {Plan} from './plan.service';
import {
	isCheckoutCatalogPlanKey,
	isEnterprisePlanKey,
	plansMatch,
	resolvePlanKey
} from './plan-identity.util';

const PRODUCT_TYPE_SELF_HOSTED = 'self_hosted';
const PRODUCT_TYPE_CLOUD = 'cloud';

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
		return isEnterprisePlanKey(plan);
	}

	shouldContactForMissingCloudPlan(plan: Plan, isSelfHostedBilling: boolean, planExistsInCatalog: boolean): boolean {
		return !isSelfHostedBilling && !planExistsInCatalog && isCheckoutCatalogPlanKey(plan);
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
		const planKey = resolvePlanKey(plan);
		if (planKey) {
			const byKey = defaultPlans.find(defaultPlan => resolvePlanKey(defaultPlan) === planKey);
			if (byKey) {
				return byKey;
			}
		}

		return defaultPlans.find(
			defaultPlan => defaultPlan.name.trim().toLowerCase() === plan.name.trim().toLowerCase()
		);
	}

	findBillingPlanForDefault(defaultPlan: Plan, billingPlans: Plan[]): Plan | undefined {
		const defaultKey = resolvePlanKey(defaultPlan);
		if (defaultKey) {
			const byKey = billingPlans.find(plan => resolvePlanKey(plan) === defaultKey);
			if (byKey) {
				return byKey;
			}
		}

		return billingPlans.find(
			plan => plan.name.trim().toLowerCase() === defaultPlan.name.trim().toLowerCase()
		);
	}

	resolvePlanForApi(selectedPlanData: Plan, billingPlans: Plan[]): ResolvedPlanForApi {
		const billingPlan = billingPlans.find(plan => plansMatch(plan, selectedPlanData));

		return {
			planExistsInCatalog: !!billingPlan,
			planIdForApi: billingPlan?.id ?? selectedPlanData.id
		};
	}

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
