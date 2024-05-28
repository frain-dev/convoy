import { TestBed } from '@angular/core/testing';

import { PortalService } from './portal.service';

describe('PortalService', () => {
  let service: PortalService;

  beforeEach(() => {
    TestBed.configureTestingModule({});
    service = TestBed.inject(PortalService);
  });

  it('should be created', () => {
    expect(service).toBeTruthy();
  });
});
