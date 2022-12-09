import { TestBed } from '@angular/core/testing';

import { PortalLinksService } from './portal-links.service';

describe('PortalLinksService', () => {
  let service: PortalLinksService;

  beforeEach(() => {
    TestBed.configureTestingModule({});
    service = TestBed.inject(PortalLinksService);
  });

  it('should be created', () => {
    expect(service).toBeTruthy();
  });
});
