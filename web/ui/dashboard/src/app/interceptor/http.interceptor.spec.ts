import { TestBed } from '@angular/core/testing';
import { RouterTestingModule } from '@angular/router/testing';

import { HttpIntercepter } from './http.interceptor';

describe('HttpIntercepter', () => {
	beforeEach(() =>
		TestBed.configureTestingModule({
			providers: [HttpIntercepter],
            imports: [RouterTestingModule]
		})
	);

	it('should be created', () => {
		const interceptor: HttpIntercepter = TestBed.inject(HttpIntercepter);
		expect(interceptor).toBeTruthy();
	});
});
