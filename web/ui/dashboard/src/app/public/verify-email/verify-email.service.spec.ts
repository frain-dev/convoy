import { TestBed } from '@angular/core/testing';

import { VerifyEmailService } from './verify-email.service';

describe('VerifyEmailService', () => {
  let service: VerifyEmailService;

  beforeEach(() => {
    TestBed.configureTestingModule({});
    service = TestBed.inject(VerifyEmailService);
  });

  it('should be created', () => {
    expect(service).toBeTruthy();
  });
});
