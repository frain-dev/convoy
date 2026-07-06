import { of } from 'rxjs';

import { HttpService } from './http.service';

// Constructed directly (not via TestBed): buildRequestQuery only needs the
// ActivatedRoute queryParams subscription from the constructor.
function createService(): HttpService {
	const routeStub: any = { queryParams: of({}) };
	return new HttpService({} as any, {} as any, routeStub, {} as any);
}

describe('HttpService', () => {
	let service: HttpService;

	beforeEach(() => {
		service = createService();
	});

	it('should be created', () => {
		expect(service).toBeTruthy();
	});

	describe('buildRequestQuery', () => {
		it('returns an empty string for no query', () => {
			expect(service.buildRequestQuery()).toBe('');
		});

		it('serializes scalar values', () => {
			expect(service.buildRequestQuery({ perPage: 20, direction: 'next' })).toBe('perPage=20&direction=next');
		});

		it('expands a JSON-array string into repeated keys', () => {
			expect(service.buildRequestQuery({ status: '["Failure","Retry"]' })).toBe('status=Failure&status=Retry');
		});

		// Regression: repeated keys from a JSON-array value in the LAST position used
		// to be concatenated without '&' (status=Failurestatus=Retry), so the backend
		// matched nothing and batch retry counted 0.
		it('keeps & separators when the JSON-array value is the last key', () => {
			expect(service.buildRequestQuery({ perPage: 20, status: '["Failure","Retry"]' })).toBe('perPage=20&status=Failure&status=Retry');
		});

		it('keeps & separators when the JSON-array value is the first key', () => {
			expect(service.buildRequestQuery({ status: '["Failure","Retry"]', perPage: 20 })).toBe('status=Failure&status=Retry&perPage=20');
		});

		it('expands real array values into repeated keys', () => {
			expect(service.buildRequestQuery({ perPage: 20, endpointId: ['ep1', 'ep2'] })).toBe('perPage=20&endpointId=ep1&endpointId=ep2');
		});

		it('drops empty, null and undefined values', () => {
			expect(service.buildRequestQuery({ q: '', a: null, b: undefined, perPage: 20 })).toBe('perPage=20');
		});

		it('appends the portal token when set', () => {
			service.token = 'ptl123';
			expect(service.buildRequestQuery({ perPage: 20 })).toBe('perPage=20&token=ptl123');
		});
	});
});
