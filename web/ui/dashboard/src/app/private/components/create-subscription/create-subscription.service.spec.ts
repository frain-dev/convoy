import { TestBed } from '@angular/core/testing';
import { RouterTestingModule } from '@angular/router/testing';

import { CreateSubscriptionService } from './create-subscription.service';

describe('CreateSubscriptionService', () => {
	let service: CreateSubscriptionService;

	beforeEach(() => {
		TestBed.configureTestingModule({
            imports: [RouterTestingModule]
        });
		service = TestBed.inject(CreateSubscriptionService);
	});

	it('should be created', () => {
		expect(service).toBeTruthy();
	});
});
