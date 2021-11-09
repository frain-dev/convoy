import { TestBed } from '@angular/core/testing';

import { GeneralService } from './general.service';

describe('GeneralService', () => {
  let service: GeneralService;

  beforeEach(() => {
    TestBed.configureTestingModule({});
    service = TestBed.inject(GeneralService);
  });

  it('should be created', () => {
    expect(service).toBeTruthy();
  });
});
