import { of } from 'rxjs';

import { consumeInviteRedirect } from 'src/app/public/accept-invite/invite-redirect';
import { HttpService } from './http.service';

// Constructed directly (not via TestBed): buildRequestQuery only needs the
// ActivatedRoute queryParams subscription from the constructor.
function createService(router: any = {}): HttpService {
	const routeStub: any = { queryParams: of({}) };
	return new HttpService(router, {} as any, routeStub, {} as any);
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

		it('percent-encodes search terms with spaces and punctuation', () => {
			expect(service.buildRequestQuery({ query: 'feat: benchmark commit 552' })).toBe(
				'query=feat%3A%20benchmark%20commit%20552'
			);
		});

		it('percent-encodes json object search strings', () => {
			expect(service.buildRequestQuery({ query: '{"status":"paid"}' })).toBe('query=%7B%22status%22%3A%22paid%22%7D');
		});

		it('does not json-expand the query param', () => {
			expect(service.buildRequestQuery({ query: '["only-one-term"]' })).toBe('query=%5B%22only-one-term%22%5D');
		});
	});

	describe('logUserOut', () => {
		beforeEach(() => {
			localStorage.clear();
		});

		afterEach(() => {
			localStorage.clear();
		});

		it('keeps the accept-invite token so login can return', () => {
			const router = {
				url: '/accept-invite?invite-token=abcDEF12',
				navigate: jasmine.createSpy('navigate')
			};
			const http = createService(router);
			http.logUserOut();
			expect(consumeInviteRedirect()).toBe('/accept-invite?invite-token=abcDEF12');
			expect(router.navigate).toHaveBeenCalledWith(['/login'], { replaceUrl: true });
		});

		it('does not store a non-invite location as the invite redirect', () => {
			const router = {
				url: '/projects',
				navigate: jasmine.createSpy('navigate')
			};
			const http = createService(router);
			http.logUserOut();
			expect(consumeInviteRedirect()).toBeNull();
		});
	});
});
