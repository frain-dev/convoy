import { resolveCheckoutCadence } from './plan-cadence.util';
import { Plan } from './plan.service';

export const BILLING_PLANS_UNAVAILABLE_MESSAGE = 'Billing plans unavailable. Retry.';
export const CLOUD_MANAGED_PLANS_MESSAGE = 'Plans are managed in Cloud for this organisation. Open the cloud workspace billing page to manage plans.';
export const SELF_HOSTED_MANAGED_PLANS_MESSAGE = 'Plans for this organisation are only available in self-hosted billing mode.';
const SELF_HOSTED_PRODUCT_TYPE = 'self_hosted';
const CLOUD_PRODUCT_TYPE = 'cloud';
export type BillingPlansUnavailableReason = 'none' | 'fetch_error' | 'empty_catalog' | 'mode_filtered';

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

export function resolveBillingPlansUnavailableMessage(
  reason: BillingPlansUnavailableReason,
  selfHostedBilling: boolean
): string {
  if (reason === 'mode_filtered') {
    return selfHostedBilling ? SELF_HOSTED_MANAGED_PLANS_MESSAGE : CLOUD_MANAGED_PLANS_MESSAGE;
  }

  if (reason === 'empty_catalog' || reason === 'fetch_error') {
    return BILLING_PLANS_UNAVAILABLE_MESSAGE;
  }

  return '';
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
