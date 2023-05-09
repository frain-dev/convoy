import { TestBed } from '@angular/core/testing';

import { MetaEventsService } from './meta-events.service';

describe('MetaEventsService', () => {
  let service: MetaEventsService;

  beforeEach(() => {
    TestBed.configureTestingModule({});
    service = TestBed.inject(MetaEventsService);
  });

  it('should be created', () => {
    expect(service).toBeTruthy();
  });
});
