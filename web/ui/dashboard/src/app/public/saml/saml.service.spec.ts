import { TestBed } from '@angular/core/testing';

import { SamlService } from './saml.service';

describe('SamlService', () => {
  let service: SamlService;

  beforeEach(() => {
    TestBed.configureTestingModule({});
    service = TestBed.inject(SamlService);
  });

  it('should be created', () => {
    expect(service).toBeTruthy();
  });
});
