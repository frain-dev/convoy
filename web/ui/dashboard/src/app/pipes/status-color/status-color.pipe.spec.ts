import { StatusColorPipe } from '../status-color/status-color.pipe';

describe('StatusColorPipe', () => {
	const pipe = new StatusColorPipe();

	it('create an instance', () => {
		expect(pipe).toBeTruthy();
	});

	it('maps delivery outcomes without treating Discarded as a failure', () => {
		expect(pipe.transform('Success')).toBe('success');
		expect(pipe.transform('Failure')).toBe('error');
		expect(pipe.transform('Discarded')).toBe('neutral');
		expect(pipe.transform('Scheduled')).toBe('neutral');
		expect(pipe.transform('Processing')).toBe('neutral');
		expect(pipe.transform('Retry')).toBe('neutral');
	});
});
