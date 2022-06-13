import { TestBed } from '@angular/core/testing';

import { EventDeliveryDetailsService } from './event-delivery-details.service';

describe('EventDeliveryDetailsService', () => {
  let service: EventDeliveryDetailsService;

  beforeEach(() => {
    TestBed.configureTestingModule({});
    service = TestBed.inject(EventDeliveryDetailsService);
  });

  it('should be created', () => {
    expect(service).toBeTruthy();
  });
});
