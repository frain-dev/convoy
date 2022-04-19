import { HttpClientModule, HttpClient } from '@angular/common/http';
import { TestBed } from '@angular/core/testing';

import { ConvoyDashboardService } from './convoy-dashboard.service';

describe('ConvoyDashboardService', () => {
	let service: ConvoyDashboardService;

	beforeEach(() => {
		TestBed.configureTestingModule({
			imports: [HttpClientModule],
			providers: [HttpClient]
		});
		service = TestBed.inject(ConvoyDashboardService);
	});

	it('should be created', () => {
		expect(service).toBeTruthy();
	});
});
