import {
  areOverwatchPlansAvailable,
  BILLING_PLANS_UNAVAILABLE_MESSAGE,
  mapOverwatchPlansForCheckout,
  shouldFetchPlans
} from './billing-plans.util';
import { Plan } from './plan.service';

describe('billing plans helpers', () => {
  const basePlan: Plan = {
    id: 'plan_pro',
    name: 'Pro',
    description: 'Pro plan',
    price: 99,
    currency: 'USD',
    interval: 'month',
    intervals: ['monthly'],
    pricing_options: [{ interval: 'annual' }],
    features: []
  };

  it('maps checkout plans from Overwatch data only', () => {
    const mapped = mapOverwatchPlansForCheckout([basePlan]);
    expect(mapped.length).toBe(1);
    expect(mapped[0].id).toBe('plan_pro');
    expect(mapped[0].interval).toBe('annual');
  });

  it('returns empty plans when response is empty', () => {
    expect(mapOverwatchPlansForCheckout([])).toEqual([]);
    expect(mapOverwatchPlansForCheckout(null)).toEqual([]);
  });

  it('only fetches plans when needed', () => {
    expect(shouldFetchPlans(false, false, false)).toBeTrue();
    expect(shouldFetchPlans(true, false, false)).toBeFalse();
    expect(shouldFetchPlans(true, false, true)).toBeTrue();
    expect(shouldFetchPlans(false, true, true)).toBeFalse();
  });

  it('flags checkout as unavailable with no plans', () => {
    expect(areOverwatchPlansAvailable([])).toBeFalse();
    expect(areOverwatchPlansAvailable([basePlan])).toBeTrue();
    expect(BILLING_PLANS_UNAVAILABLE_MESSAGE).toBe('Billing plans unavailable. Retry.');
  });
});
