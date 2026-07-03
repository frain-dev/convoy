import {BillingOverviewService, formatTimeUntilReset} from './billing-overview.service';

describe('BillingOverviewService.formatOverviewData', () => {
	let service: BillingOverviewService;
	// Fixed "now" so reset countdown math is deterministic. millisecondsUntilDate
	// reads Date.now()/new Date(), both driven by the jasmine clock below.
	const baseNow = new Date('2026-01-01T00:00:00Z');

	beforeEach(() => {
		jasmine.clock().install();
		jasmine.clock().mockDate(baseNow);
		// formatOverviewData does not touch the http service, so a bare stub is enough.
		service = new BillingOverviewService({} as any);
	});

	afterEach(() => jasmine.clock().uninstall());

	function subscription(overrides: Record<string, any> = {}) {
		return {
			id: 'sub_1',
			status: 'active',
			plan: { id: 'plan_1', name: 'Cloud Pro', price: 199 },
			...overrides
		};
	}

	describe('trial badge', () => {
		it('flags the plan as trialing', () => {
			const overview = service.formatOverviewData({
				subscription: subscription({ trial: true, trial_conversion_date: '2026-01-06T00:00:00Z' }),
				usage: null,
				payment: null
			});

			expect(overview.plan.trial).toBeTrue();
		});

		it('uses the trial window for usage period instead of a long billing cycle', () => {
			const overview = service.formatOverviewData({
				subscription: subscription({
					trial: true,
					trial_conversion_date: '2026-01-15T00:00:00Z',
					current_period_start: '2024-07-03T00:00:00Z',
					current_period_end: '2026-07-03T00:00:00Z',
					next_invoice_date: '2026-07-03T00:00:00Z'
				}),
				usage: null,
				payment: null
			});

			expect(overview.usage.period).not.toContain('2024');
			expect(overview.usage.period).toContain('Jan');
			expect(overview.usage.resetIn).toBe('14 days');
		});

		it('does not flag a non-trial subscription', () => {
			const overview = service.formatOverviewData({
				subscription: subscription(),
				usage: null,
				payment: null
			});

			expect(overview.plan.trial).toBeFalse();
		});
	});

	describe('past-due / suspended detection', () => {
		it('flags a paused subscription as past due', () => {
			const overview = service.formatOverviewData({
				subscription: subscription({ status: 'paused' }),
				usage: null,
				payment: null
			});

			expect(overview.pastDue).toBeTrue();
		});

		it('normalizes casing and separators (e.g. "Past Due")', () => {
			const overview = service.formatOverviewData({
				subscription: subscription({ status: 'Past Due' }),
				usage: null,
				payment: null
			});

			expect(overview.pastDue).toBeTrue();
		});

		it('does not flag a healthy active subscription', () => {
			const overview = service.formatOverviewData({
				subscription: subscription({ status: 'active' }),
				usage: null,
				payment: null
			});

			expect(overview.pastDue).toBeFalse();
		});

		it('fails soft (not past due) on an unknown status', () => {
			const overview = service.formatOverviewData({
				subscription: subscription({ status: 'something_new' }),
				usage: null,
				payment: null
			});

			expect(overview.pastDue).toBeFalse();
		});

		it('does not flag an org with no subscription', () => {
			const overview = service.formatOverviewData({ subscription: null, usage: null, payment: null });

			expect(overview.pastDue).toBeFalse();
		});
	});
});

describe('formatTimeUntilReset', () => {
	it('uses singular units for 1', () => {
		expect(formatTimeUntilReset(60 * 1000)).toBe('1 minute');
		expect(formatTimeUntilReset(60 * 60 * 1000)).toBe('1 hour');
		expect(formatTimeUntilReset(24 * 60 * 60 * 1000)).toBe('1 day');
		expect(formatTimeUntilReset(365 * 24 * 60 * 60 * 1000)).toBe('1 year');
	});

	it('uses plural units for values greater than 1', () => {
		expect(formatTimeUntilReset(2 * 60 * 1000)).toBe('2 minutes');
		expect(formatTimeUntilReset(3 * 60 * 60 * 1000)).toBe('3 hours');
		expect(formatTimeUntilReset(5 * 24 * 60 * 60 * 1000)).toBe('5 days');
		expect(formatTimeUntilReset(2 * 365 * 24 * 60 * 60 * 1000)).toBe('2 years');
	});
});

describe('BillingOverviewService usage reset countdown', () => {
	let service: BillingOverviewService;

	beforeEach(() => {
		service = new BillingOverviewService({} as any);
	});

	afterEach(() => {
		if (jasmine.clock) {
			jasmine.clock().uninstall();
		}
	});

	function subscription(overrides: Record<string, any> = {}) {
		return {
			id: 'sub_1',
			status: 'active',
			plan: { id: 'plan_1', name: 'Cloud Pro', price: 199 },
			current_period_start: '2026-06-03T00:00:00Z',
			current_period_end: '2026-07-03T00:00:00Z',
			next_invoice_date: '2026-07-03T00:00:00Z',
			...overrides
		};
	}

	it('formats reset from next_invoice_date with correct singular/plural units', () => {
		const baseNow = new Date('2026-07-02T00:00:00Z');
		const nextInvoice = '2026-07-03T00:00:00Z';
		jasmine.clock().install();
		jasmine.clock().mockDate(baseNow);

		const overview = service.formatOverviewData({
			subscription: subscription({ next_invoice_date: nextInvoice }),
			usage: { period: '2026-07' },
			payment: null
		});

		const expectedMs = new Date(nextInvoice).getTime() - baseNow.getTime();
		expect(overview.usage.resetIn).toBe(formatTimeUntilReset(expectedMs));
		expect(overview.usage.resetIn).toBe('1 day');
	});

	it('formats reset from usage period when cycle dates are absent', () => {
		const baseNow = new Date('2026-06-30T12:00:00Z');
		jasmine.clock().install();
		jasmine.clock().mockDate(baseNow);

		const overview = service.formatOverviewData({
			subscription: subscription({
				current_period_start: undefined,
				current_period_end: undefined,
				next_invoice_date: undefined
			}),
			usage: { period: '2026-06' },
			payment: null
		});

		const nextPeriodStart = new Date(2026, 6, 1);
		const expectedMs = nextPeriodStart.getTime() - baseNow.getTime();
		expect(overview.usage.resetIn).toBe(formatTimeUntilReset(expectedMs));
	});

	it('omits reset countdown when there is no active subscription', () => {
		const overview = service.formatOverviewData({
			subscription: null,
			usage: { period: '2026-07' },
			payment: null
		});

		expect(overview.usage.resetIn).toBeUndefined();
	});
});
