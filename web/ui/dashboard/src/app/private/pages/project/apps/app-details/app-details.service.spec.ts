import { TestBed } from '@angular/core/testing';

import { AppDetailsService } from './app-details.service';

describe('AppDetailsService', () => {
  let service: AppDetailsService;

  beforeEach(() => {
    TestBed.configureTestingModule({});
    service = TestBed.inject(AppDetailsService);
  });

  it('should be created', () => {
    expect(service).toBeTruthy();
  });
});
