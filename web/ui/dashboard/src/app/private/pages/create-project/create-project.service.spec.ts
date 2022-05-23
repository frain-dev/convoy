import { TestBed } from '@angular/core/testing';

import { CreateProjectService } from './create-project.service';

describe('CreateProjectService', () => {
  let service: CreateProjectService;

  beforeEach(() => {
    TestBed.configureTestingModule({});
    service = TestBed.inject(CreateProjectService);
  });

  it('should be created', () => {
    expect(service).toBeTruthy();
  });
});
