import { TestBed } from '@angular/core/testing';

import { HubspotService } from './hubspot.service';

describe('PosthogService', () => {
	let service: HubspotService;

	beforeEach(() => {
		TestBed.configureTestingModule({});
		service = TestBed.inject(HubspotService);
	});

	it('should be created', () => {
		expect(service).toBeTruthy();
	});
});
