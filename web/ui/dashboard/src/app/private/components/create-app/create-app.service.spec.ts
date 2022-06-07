import { TestBed } from '@angular/core/testing';

import { CreateAppService } from './create-app.service';

describe('CreateAppService', () => {
  let service: CreateAppService;

  beforeEach(() => {
    TestBed.configureTestingModule({});
    service = TestBed.inject(CreateAppService);
  });

  it('should be created', () => {
    expect(service).toBeTruthy();
  });
});
