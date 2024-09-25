import { TestBed } from '@angular/core/testing';

import { IframeGuard } from './iframe.guard';

describe('IframeGuard', () => {
  let guard: IframeGuard;

  beforeEach(() => {
    TestBed.configureTestingModule({});
    guard = TestBed.inject(IframeGuard);
  });

  it('should be created', () => {
    expect(guard).toBeTruthy();
  });
});
