import { TestBed } from '@angular/core/testing';

import { CreateSourceService } from './create-source.service';

describe('CreateSourceService', () => {
  let service: CreateSourceService;

  beforeEach(() => {
    TestBed.configureTestingModule({});
    service = TestBed.inject(CreateSourceService);
  });

  it('should be created', () => {
    expect(service).toBeTruthy();
  });
});
