import { TestBed } from '@angular/core/testing';

import { CreateEndpointService } from './create-endpoint.service';

describe('CreateEndpointService', () => {
  let service: CreateEndpointService;

  beforeEach(() => {
    TestBed.configureTestingModule({});
    service = TestBed.inject(CreateEndpointService);
  });

  it('should be created', () => {
    expect(service).toBeTruthy();
  });
});
