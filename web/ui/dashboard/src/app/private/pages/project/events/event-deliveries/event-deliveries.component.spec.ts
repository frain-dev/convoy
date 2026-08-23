import { EventDeliveriesComponent } from './event-deliveries.component';

describe('EventDeliveriesComponent status pills', () => {
	const pills = Object.create(EventDeliveriesComponent.prototype) as EventDeliveriesComponent;

	it('keeps Failure red and Discarded muted', () => {
		expect(pills.statusPillClass('Success')).toBe('bg-success-a3 text-success-11');
		expect(pills.statusPillClass('Failure')).toBe('bg-error-a3 text-error-11');
		expect(pills.statusPillClass('Discarded')).toBe('bg-new.surface-muted text-new.text-secondary');
		expect(pills.statusPillClass('Scheduled')).toBe('bg-new.surface-muted text-new.text-secondary');
	});
});
