import { TestBed } from '@angular/core/testing';

import { SignupGuard } from './signup.guard';

describe('SignupGuard', () => {
  let guard: SignupGuard;

  beforeEach(() => {
    TestBed.configureTestingModule({});
    guard = TestBed.inject(SignupGuard);
  });

  it('should be created', () => {
    expect(guard).toBeTruthy();
  });
});
