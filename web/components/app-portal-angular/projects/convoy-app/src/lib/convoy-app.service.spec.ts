import { TestBed } from '@angular/core/testing';

import { ConvoyAppService } from './convoy-app.service';

describe('ConvoyAppService', () => {
  let service: ConvoyAppService;

  beforeEach(() => {
    TestBed.configureTestingModule({});
    service = TestBed.inject(ConvoyAppService);
  });

  it('should be created', () => {
    expect(service).toBeTruthy();
  });
});
