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
		if (!defaultPlan) {
			return plan;
		}

		// Marketing card bullets never come from the billing API, so always take
		// them from the default (unless the plan already carries its own).
		const highlights = plan.highlights ?? defaultPlan.highlights;
		if (plan.features && plan.features.length > 0) {
			return { ...plan, highlights };
		}

		return {
			...plan,
			features: defaultPlan.features,
			highlights,
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
			// Cloud renders dynamically from the billing catalog (same as self-hosted):
			// the plan list is whatever the billing service returns for cloud, enriched
			// with default comparison copy/pricing since the API does not carry marketing
			// feature rows. Sorted deterministically (checkout plans by price ascending,
			// contact-only plans last) because the catalog endpoint returns no guaranteed order.
			plans = billingPlans
				.map((plan: Plan) => this.mergePlanWithDefaultComparison(plan, defaultPlans))
				.sort((a, b) => this.compareCloudPlans(a, b));
			if (plans.length === 0) {
				plansUnavailableMessage = 'Cloud plans are not available right now. Please try again later.';
			}
		}

		return { plans, billingPlans, plansUnavailableMessage };
	}

	// Single source of truth for "this plan cannot self-serve checkout" (contact
	// sales). Sort ordering and checkout gating must agree, so both go through
	// here: explicit requires_contact wins, then the checkout_enabled flag, then
	// enterprise-key fallback. A contact-only plan sorts last regardless of price.
	planRequiresContact(plan: Plan): boolean {
		if (plan.requires_contact !== undefined) {
			return plan.requires_contact;
		}
		if (plan.checkout_enabled !== undefined) {
			return !plan.checkout_enabled;
		}
		return this.isEnterprisePlan(plan);
	}

	private compareCloudPlans(a: Plan, b: Plan): number {
		const aContact = this.planRequiresContact(a);
		const bContact = this.planRequiresContact(b);
		if (aContact !== bContact) {
			return aContact ? 1 : -1;
		}
		return this.planAmountCents(a) - this.planAmountCents(b);
	}

	private planAmountCents(plan: Plan): number {
		const amounts = (plan.pricing_options || [])
			.map(option => option?.amount_cents)
			.filter((cents): cents is number => typeof cents === 'number');
		if (amounts.length > 0) {
			return Math.min(...amounts);
		}
		// `pricing_options` carries cents; `plan.price` is dollars. Convert so the
		// fallback compares in the same unit as the primary path (otherwise a
		// dollars value like 499 sorts below a cents value like 9900).
		return typeof plan.price === 'number' ? Math.round(plan.price * 100) : Number.MAX_SAFE_INTEGER;
	}
}
