import { TestBed } from '@angular/core/testing';
import { RouterTestingModule } from '@angular/router/testing';

import { EventDeliveryDetailsService } from './event-delivery-details.service';

describe('EventDeliveryDetailsService', () => {
  let service: EventDeliveryDetailsService;

  beforeEach(() => {
    TestBed.configureTestingModule({
        imports: [RouterTestingModule]
    });
    service = TestBed.inject(EventDeliveryDetailsService);
  });

  it('should be created', () => {
    expect(service).toBeTruthy();
  });
});
