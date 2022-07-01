import { TestBed } from '@angular/core/testing';

import { AddAnalyticsService } from './add-analytics.service';

describe('AddAnalyticsService', () => {
  let service: AddAnalyticsService;

  beforeEach(() => {
    TestBed.configureTestingModule({});
    service = TestBed.inject(AddAnalyticsService);
  });

  it('should be created', () => {
    expect(service).toBeTruthy();
  });
});
