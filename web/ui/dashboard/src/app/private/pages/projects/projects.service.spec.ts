import { TestBed } from '@angular/core/testing';
import { RouterTestingModule } from '@angular/router/testing';

import { ProjectsService } from '../projects/projects.service';

describe('ProjectsService', () => {
	let service: ProjectsService;

	beforeEach(() => {
		TestBed.configureTestingModule({
            imports: [RouterTestingModule]
        });
		service = TestBed.inject(ProjectsService);
	});

	it('should be created', () => {
		expect(service).toBeTruthy();
	});
});
