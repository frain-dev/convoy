import { TestBed } from '@angular/core/testing';

import { EventsCatalogueService } from './events-catalogue.service';

describe('EventsCatalogueService', () => {
  let service: EventsCatalogueService;

  beforeEach(() => {
    TestBed.configureTestingModule({});
    service = TestBed.inject(EventsCatalogueService);
  });

  it('should be created', () => {
    expect(service).toBeTruthy();
  });
});
