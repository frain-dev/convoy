export type PlanCadenceOption = {
  interval?: string | null;
};

export type PlanCadenceSource = {
  interval?: string | null;
  intervals?: string[] | null;
  pricing_options?: PlanCadenceOption[] | null;
};

const LEGACY_CADENCE_MAP: Record<string, string> = {
  month: 'monthly',
  year: 'annual'
};

export function normalizeCadence(value?: string | null): string {
  const raw = (value || '').trim().toLowerCase();
  if (!raw) {
    return '';
  }

  return LEGACY_CADENCE_MAP[raw] || raw;
}

export function resolveCheckoutCadence(plan: PlanCadenceSource): string {
  const pricingInterval = plan.pricing_options?.find(option => !!option?.interval)?.interval;
  const intervalsInterval = plan.intervals?.find(interval => !!interval);

  return (
    normalizeCadence(pricingInterval) ||
    normalizeCadence(intervalsInterval) ||
    normalizeCadence(plan.interval) ||
    'monthly'
  );
}

export function buildCheckoutPayload(planId: string, host: string, plan: PlanCadenceSource): {
  plan_id: string;
  host: string;
  interval: string;
} {
  return {
    plan_id: planId,
    host,
    interval: resolveCheckoutCadence(plan)
  };
}
