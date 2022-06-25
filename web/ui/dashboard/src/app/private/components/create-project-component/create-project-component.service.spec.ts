import { TestBed } from '@angular/core/testing';

import { CreateProjectComponentService } from './create-project-component.service';

describe('CreateProjectService', () => {
	let service: CreateProjectComponentService;

	beforeEach(() => {
		TestBed.configureTestingModule({});
		service = TestBed.inject(CreateProjectComponentService);
	});

	it('should be created', () => {
		expect(service).toBeTruthy();
	});
});
