import { isConvoyCloud } from './cloud.util';

describe('isConvoyCloud', () => {
	it('matches regional, staging and tenant cloud hosts', () => {
		expect(isConvoyCloud('us.getconvoy.cloud')).toBe(true);
		expect(isConvoyCloud('eu.getconvoy.cloud')).toBe(true);
		expect(isConvoyCloud('staging.getconvoy.cloud')).toBe(true);
		expect(isConvoyCloud('curacel.getconvoy.cloud')).toBe(true);
	});

	it('does not match self-hosted or retired hosts', () => {
		expect(isConvoyCloud('convoy.acme.internal')).toBe(false);
		expect(isConvoyCloud('localhost')).toBe(false);
		expect(isConvoyCloud('dashboard.getconvoy.io')).toBe(false);
	});

	it('does not match a lookalike domain outside our zone', () => {
		expect(isConvoyCloud('notgetconvoy.cloud')).toBe(false);
		expect(isConvoyCloud('evil-getconvoy.cloud')).toBe(false);
	});
});
