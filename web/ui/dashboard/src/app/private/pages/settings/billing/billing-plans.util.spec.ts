import {
  areOverwatchPlansAvailable,
  BILLING_PLANS_UNAVAILABLE_MESSAGE,
  CLOUD_MANAGED_PLANS_MESSAGE,
  SELF_HOSTED_MANAGED_PLANS_MESSAGE,
  mapOverwatchPlansForCheckout,
  resolveBillingPlansUnavailableMessage,
  scopePlansForBillingMode,
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

  it('returns mode-aware unavailable messages', () => {
    expect(resolveBillingPlansUnavailableMessage('mode_filtered', false)).toBe(CLOUD_MANAGED_PLANS_MESSAGE);
    expect(resolveBillingPlansUnavailableMessage('mode_filtered', true)).toBe(SELF_HOSTED_MANAGED_PLANS_MESSAGE);
    expect(resolveBillingPlansUnavailableMessage('fetch_error', false)).toBe(BILLING_PLANS_UNAVAILABLE_MESSAGE);
    expect(resolveBillingPlansUnavailableMessage('empty_catalog', true)).toBe(BILLING_PLANS_UNAVAILABLE_MESSAGE);
    expect(resolveBillingPlansUnavailableMessage('none', false)).toBe('');
  });

  it('scopes plans by product_type when present', () => {
    const cloudPlan: Plan = { ...basePlan, id: 'cloud_pro', product_type: 'cloud' };
    const selfHostedPlan: Plan = { ...basePlan, id: 'self_pro', product_type: 'self_hosted' };

    expect(scopePlansForBillingMode([cloudPlan, selfHostedPlan], false)).toEqual([cloudPlan]);
    expect(scopePlansForBillingMode([cloudPlan, selfHostedPlan], true)).toEqual([selfHostedPlan]);
  });

  it('normalizes product_type values for mode scoping', () => {
    const cloudPlan: Plan = { ...basePlan, id: 'cloud_business', product_type: ' CLOUD ' };
    const selfHostedPlan: Plan = { ...basePlan, id: 'self_enterprise', product_type: 'self-hosted' };

    expect(scopePlansForBillingMode([cloudPlan, selfHostedPlan], false)).toEqual([cloudPlan]);
    expect(scopePlansForBillingMode([cloudPlan, selfHostedPlan], true)).toEqual([selfHostedPlan]);
  });

  it('excludes plans without product_type in both modes', () => {
    const cloudPlan: Plan = { ...basePlan, id: 'pro', name: 'Pro' };
    const selfHostedPlan: Plan = { ...basePlan, id: 'self_hosted_premium', name: 'Self-Hosted Premium' };

    expect(scopePlansForBillingMode([cloudPlan, selfHostedPlan], false)).toEqual([]);
    expect(scopePlansForBillingMode([cloudPlan, selfHostedPlan], true)).toEqual([]);
  });
});
