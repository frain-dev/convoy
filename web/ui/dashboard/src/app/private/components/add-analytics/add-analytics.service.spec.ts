import { TestBed } from '@angular/core/testing';
import { RouterTestingModule } from '@angular/router/testing';

import { AddAnalyticsService } from './add-analytics.service';

describe('AddAnalyticsService', () => {
  let service: AddAnalyticsService;

  beforeEach(() => {
    TestBed.configureTestingModule({
        imports: [RouterTestingModule]
    });
    service = TestBed.inject(AddAnalyticsService);
  });

  it('should be created', () => {
    expect(service).toBeTruthy();
  });
});
