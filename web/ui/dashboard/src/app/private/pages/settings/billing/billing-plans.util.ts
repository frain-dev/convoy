import { resolveCheckoutCadence } from './plan-cadence.util';
import { Plan } from './plan.service';

export const BILLING_PLANS_UNAVAILABLE_MESSAGE = 'Billing plans unavailable. Retry.';

export function mapOverwatchPlansForCheckout(plans: Plan[] | null | undefined): Plan[] {
  if (!Array.isArray(plans) || plans.length === 0) {
    return [];
  }

  return plans.map((plan: Plan) => ({
    ...plan,
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
