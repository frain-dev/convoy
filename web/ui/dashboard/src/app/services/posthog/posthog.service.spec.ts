import { TestBed } from '@angular/core/testing';

import { PosthogService } from './posthog.service';

describe('PosthogService', () => {
  let service: PosthogService;

  beforeEach(() => {
    TestBed.configureTestingModule({});
    service = TestBed.inject(PosthogService);
  });

  it('should be created', () => {
    expect(service).toBeTruthy();
  });
});
