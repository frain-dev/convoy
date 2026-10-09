import { CountLabelPipe } from './count-label.pipe';

describe('CountLabelPipe', () => {
	const pipe = new CountLabelPipe();

	it('uses the singular label when the count is 1', () => {
		expect(pipe.transform(1, 'Event sent', 'Events sent')).toBe('Event sent');
	});

	it('uses the plural label when the count is 0', () => {
		expect(pipe.transform(0, 'Failed delivery', 'Failed deliveries')).toBe('Failed deliveries');
	});

	it('uses the plural label when the count is greater than 1', () => {
		expect(pipe.transform(2, 'Active endpoint', 'Active endpoints')).toBe('Active endpoints');
	});

	it('uses the plural label when the count is unknown', () => {
		expect(pipe.transform(null, 'Successful delivery', 'Successful deliveries')).toBe('Successful deliveries');
		expect(pipe.transform(undefined, 'Successful delivery', 'Successful deliveries')).toBe('Successful deliveries');
	});
});
