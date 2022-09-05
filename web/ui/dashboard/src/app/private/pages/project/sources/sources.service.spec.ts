import { TestBed } from '@angular/core/testing';
import { RouterTestingModule } from '@angular/router/testing';

import { SourcesService } from './sources.service';

describe('SourcesService', () => {
  let service: SourcesService;

  beforeEach(() => {
    TestBed.configureTestingModule({
        imports:[RouterTestingModule]
    });
    service = TestBed.inject(SourcesService);
  });

  it('should be created', () => {
    expect(service).toBeTruthy();
  });
});
