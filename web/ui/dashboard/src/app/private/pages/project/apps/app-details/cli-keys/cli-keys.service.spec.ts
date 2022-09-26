import { TestBed } from '@angular/core/testing';

import { CliKeysService } from './cli-keys.service';

describe('CliKeysService', () => {
  let service: CliKeysService;

  beforeEach(() => {
    TestBed.configureTestingModule({});
    service = TestBed.inject(CliKeysService);
  });

  it('should be created', () => {
    expect(service).toBeTruthy();
  });
});
