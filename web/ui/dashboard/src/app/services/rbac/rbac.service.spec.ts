import { TestBed } from '@angular/core/testing';

import { RbacService } from './rbac.service';

describe('RbacService', () => {
  let service: RbacService;

  beforeEach(() => {
    TestBed.configureTestingModule({});
    service = TestBed.inject(RbacService);
  });

  it('should be created', () => {
    expect(service).toBeTruthy();
  });
});
