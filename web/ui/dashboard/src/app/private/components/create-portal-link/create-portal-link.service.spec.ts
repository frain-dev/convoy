import { TestBed } from '@angular/core/testing';

import { CreatePortalLinkService } from './create-portal-link.service';

describe('CreatePortalLinkService', () => {
  let service: CreatePortalLinkService;

  beforeEach(() => {
    TestBed.configureTestingModule({});
    service = TestBed.inject(CreatePortalLinkService);
  });

  it('should be created', () => {
    expect(service).toBeTruthy();
  });
});
