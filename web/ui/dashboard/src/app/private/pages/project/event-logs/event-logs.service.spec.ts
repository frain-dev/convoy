import { TestBed } from '@angular/core/testing';

import { EventLogsService } from './event-logs.service';

describe('EventLogsService', () => {
  let service: EventLogsService;

  beforeEach(() => {
    TestBed.configureTestingModule({});
    service = TestBed.inject(EventLogsService);
  });

  it('should be created', () => {
    expect(service).toBeTruthy();
  });
});
