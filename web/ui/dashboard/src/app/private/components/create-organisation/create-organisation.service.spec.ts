import { TestBed } from '@angular/core/testing';
import { RouterTestingModule } from '@angular/router/testing';

import { CreateOrganisationService } from './create-organisation.service';

describe('CreateOrganisationService', () => {
	let service: CreateOrganisationService;

	beforeEach(() => {
		TestBed.configureTestingModule({
			imports: [RouterTestingModule]
		});
		service = TestBed.inject(CreateOrganisationService);
	});

	it('should be created', () => {
		expect(service).toBeTruthy();
	});
});
