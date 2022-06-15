import { TestBed } from '@angular/core/testing';

import { SourcesService } from './sources.service';

describe('SourcesService', () => {
  let service: SourcesService;

  beforeEach(() => {
    TestBed.configureTestingModule({});
    service = TestBed.inject(SourcesService);
  });

  it('should be created', () => {
    expect(service).toBeTruthy();
  });
});
