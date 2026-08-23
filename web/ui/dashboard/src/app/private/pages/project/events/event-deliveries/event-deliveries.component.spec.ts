import { EventDeliveriesComponent, rollupCanServeDisplayCount, sumRollupDisplayCount } from './event-deliveries.component';

describe('EventDeliveriesComponent status pills', () => {
	const pills = Object.create(EventDeliveriesComponent.prototype) as EventDeliveriesComponent;

	it('keeps Failure red and Discarded muted', () => {
		expect(pills.statusPillClass('Success')).toBe('bg-success-a3 text-success-11');
		expect(pills.statusPillClass('Failure')).toBe('bg-error-a3 text-error-11');
		expect(pills.statusPillClass('Discarded')).toBe('bg-new.surface-muted text-new.text-secondary');
		expect(pills.statusPillClass('Scheduled')).toBe('bg-new.surface-muted text-new.text-secondary');
	});
});

describe('rollup display count', () => {
	it('sums every status when none are selected', () => {
		expect(sumRollupDisplayCount({ Success: 753161, Processing: 22, Scheduled: 6 }, [])).toBe(753189);
	});

	it('sums only the selected statuses', () => {
		expect(sumRollupDisplayCount({ Success: 753161, Processing: 22, Scheduled: 6 }, ['Scheduled', 'Processing', 'Discarded'])).toBe(28);
	});

	it('treats a missing selected status as zero', () => {
		expect(sumRollupDisplayCount({ Success: 10 }, ['Failure'])).toBe(0);
	});

	it('serves the rollup for date and endpoint filters', () => {
		expect(rollupCanServeDisplayCount({})).toBe(true);
		expect(rollupCanServeDisplayCount({ query: '', eventId: '  ', eventType: undefined })).toBe(true);
	});

	it('omits N when the filter is live-only', () => {
		expect(rollupCanServeDisplayCount({ query: 'invoice.paid' })).toBe(false);
		expect(rollupCanServeDisplayCount({ eventId: '01M0A9MCES4EG2W1JGR41VFXJQ' })).toBe(false);
		expect(rollupCanServeDisplayCount({ eventType: 'invoice.paid' })).toBe(false);
	});
});
