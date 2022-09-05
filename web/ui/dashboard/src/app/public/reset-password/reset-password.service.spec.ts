import { TestBed } from '@angular/core/testing';
import { RouterTestingModule } from '@angular/router/testing';

import { ResetPasswordService } from './reset-password.service';

describe('ResetPasswordService', () => {
	let service: ResetPasswordService;

	beforeEach(() => {
		TestBed.configureTestingModule({
			imports: [RouterTestingModule]
		});
		service = TestBed.inject(ResetPasswordService);
	});

	it('should be created', () => {
		expect(service).toBeTruthy();
	});
});
