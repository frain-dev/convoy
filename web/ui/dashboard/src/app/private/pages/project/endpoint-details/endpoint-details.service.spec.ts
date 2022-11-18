import { TestBed } from '@angular/core/testing';

import { EndpointDetailsService } from './endpoint-details.service';

describe('EndpointDetailsService', () => {
  let service: EndpointDetailsService;

  beforeEach(() => {
    TestBed.configureTestingModule({});
    service = TestBed.inject(EndpointDetailsService);
  });

  it('should be created', () => {
    expect(service).toBeTruthy();
  });
});
