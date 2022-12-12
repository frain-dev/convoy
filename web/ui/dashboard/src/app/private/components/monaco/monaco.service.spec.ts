import { TestBed } from '@angular/core/testing';

import { MonacoService } from './monaco.service';

describe('MonacoService', () => {
  let service: MonacoService;

  beforeEach(() => {
    TestBed.configureTestingModule({});
    service = TestBed.inject(MonacoService);
  });

  it('should be created', () => {
    expect(service).toBeTruthy();
  });
});
