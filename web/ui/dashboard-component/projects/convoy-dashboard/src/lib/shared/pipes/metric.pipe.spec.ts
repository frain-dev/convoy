import { MetricPipe } from '.';

describe('MetricPipe: instantialization', () => {
	it('create an instance', () => {
		const pipe = new MetricPipe();
		expect(pipe).toBeTruthy();
	});
});

describe('MetricPipe', () => {
	let pipe: MetricPipe;

	beforeEach(() => {
		pipe = new MetricPipe();
	});

	it('should format numbers: 1000', () => {
		expect(pipe.transform(1000)).toEqual('1,000');
	});

	it('should format numbers: 10001', () => {
		expect(pipe.transform(10001)).toEqual('10,001');
	});

	it('should format numbers: 120000', () => {
		expect(pipe.transform(120000)).toEqual('120,000');
	});
});
