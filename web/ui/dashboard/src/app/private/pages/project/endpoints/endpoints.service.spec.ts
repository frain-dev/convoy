import { TestBed } from '@angular/core/testing';

import { EndpointsService } from './endpoints.service';

describe('EndpointsService', () => {
  let service: EndpointsService;

  beforeEach(() => {
    TestBed.configureTestingModule({});
    service = TestBed.inject(EndpointsService);
  });

  it('should be created', () => {
    expect(service).toBeTruthy();
  });
});
