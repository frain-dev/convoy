import { TestBed } from '@angular/core/testing';

import { CreateOrganisationService } from './create-organisation.service';

describe('CreateOrganisationService', () => {
  let service: CreateOrganisationService;

  beforeEach(() => {
    TestBed.configureTestingModule({});
    service = TestBed.inject(CreateOrganisationService);
  });

  it('should be created', () => {
    expect(service).toBeTruthy();
  });
});
