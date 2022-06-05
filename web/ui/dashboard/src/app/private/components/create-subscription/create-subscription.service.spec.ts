import { TestBed } from '@angular/core/testing';

import { CreateSubscriptionService } from './create-subscription.service';

describe('CreateSubscriptionService', () => {
	let service: CreateSubscriptionService;

	beforeEach(() => {
		TestBed.configureTestingModule({});
		service = TestBed.inject(CreateSubscriptionService);
	});

	it('should be created', () => {
		expect(service).toBeTruthy();
	});
});
