import { TestBed } from '@angular/core/testing';

import { HttpIntercepter } from './http.interceptor';

describe('HttpIntercepter', () => {
	beforeEach(() =>
		TestBed.configureTestingModule({
			providers: [HttpIntercepter]
		})
	);

	it('should be created', () => {
		const interceptor: HttpIntercepter = TestBed.inject(HttpIntercepter);
		expect(interceptor).toBeTruthy();
	});
});
