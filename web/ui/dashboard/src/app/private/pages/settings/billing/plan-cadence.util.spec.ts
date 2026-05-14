import { buildCheckoutPayload, normalizeCadence, resolveCheckoutCadence } from './plan-cadence.util';

describe('plan cadence helpers', () => {
  it('keeps canonical overwatch cadence values unchanged', () => {
    expect(normalizeCadence('monthly')).toBe('monthly');
    expect(normalizeCadence('annual')).toBe('annual');
  });

  it('normalizes legacy fallback cadence values', () => {
    expect(normalizeCadence('month')).toBe('monthly');
    expect(normalizeCadence('year')).toBe('annual');
  });

  it('prefers cadence from pricing options', () => {
    const cadence = resolveCheckoutCadence({
      interval: 'month',
      intervals: ['monthly'],
      pricing_options: [{ interval: 'annual' }]
    });

    expect(cadence).toBe('annual');
  });

  it('falls back to intervals then interval and defaults to monthly', () => {
    expect(resolveCheckoutCadence({ intervals: ['annual'] })).toBe('annual');
    expect(resolveCheckoutCadence({ interval: 'month' })).toBe('monthly');
    expect(resolveCheckoutCadence({})).toBe('monthly');
  });

  it('builds payload with normalized cadence', () => {
    const payload = buildCheckoutPayload('plan_123', 'https://example.com', {
      interval: 'year'
    });

    expect(payload).toEqual({
      plan_id: 'plan_123',
      host: 'https://example.com',
      interval: 'annual'
    });
  });
});
