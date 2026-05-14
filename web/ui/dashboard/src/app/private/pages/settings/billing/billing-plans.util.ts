import { resolveCheckoutCadence } from './plan-cadence.util';
import { Plan } from './plan.service';

export const BILLING_PLANS_UNAVAILABLE_MESSAGE = 'Billing plans unavailable. Retry.';
const SELF_HOSTED_PRODUCT_TYPE = 'self_hosted';
const CLOUD_PRODUCT_TYPE = 'cloud';

export function mapOverwatchPlansForCheckout(plans: Plan[] | null | undefined): Plan[] {
  if (!Array.isArray(plans) || plans.length === 0) {
    return [];
  }

  return plans.map((plan: Plan) => ({
    ...plan,
    features: Array.isArray(plan.features) ? plan.features : [],
    interval: resolveCheckoutCadence(plan)
  }));
}

export function shouldFetchPlans(hasLoadedPlans: boolean, isLoadingPlans: boolean, forceReload: boolean): boolean {
  if (isLoadingPlans) {
    return false;
  }

  if (hasLoadedPlans && !forceReload) {
    return false;
  }

  return true;
}

export function areOverwatchPlansAvailable(plans: Plan[]): boolean {
  return plans.length > 0;
}

export function scopePlansForBillingMode(plans: Plan[] | null | undefined, selfHostedBilling: boolean): Plan[] {
  if (!Array.isArray(plans) || plans.length === 0) {
    return [];
  }

  return plans.filter(plan => isPlanInBillingMode(plan, selfHostedBilling));
}

function isPlanInBillingMode(plan: Plan, selfHostedBilling: boolean): boolean {
  const normalizedProductType = normalizePlanValue(plan.product_type);
  if (!normalizedProductType) {
    return false;
  }

  return selfHostedBilling
    ? normalizedProductType === SELF_HOSTED_PRODUCT_TYPE
    : normalizedProductType === CLOUD_PRODUCT_TYPE;
}

function normalizePlanValue(value: string | undefined | null): string {
  return (value || '')
    .trim()
    .toLowerCase()
    .replace(/[-\s]+/g, '_')
    .replace(/^_+|_+$/g, '');
}
