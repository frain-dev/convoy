import { TestBed } from '@angular/core/testing';
import { RouterTestingModule } from '@angular/router/testing';

import { SettingsService } from './settings.service';

describe('SettingsService', () => {
	let service: SettingsService;

	beforeEach(() => {
		TestBed.configureTestingModule({
			imports: [RouterTestingModule]
		});
		service = TestBed.inject(SettingsService);
	});

	it('should be created', () => {
		expect(service).toBeTruthy();
	});
});
