import { TestBed } from '@angular/core/testing';
import { RouterTestingModule } from '@angular/router/testing';

import { AcceptInviteService } from './accept-invite.service';

describe('AcceptInviteService', () => {
  let service: AcceptInviteService;

  beforeEach(() => {
    TestBed.configureTestingModule({
        imports: [RouterTestingModule]
    });
    service = TestBed.inject(AcceptInviteService);
  });

  it('should be created', () => {
    expect(service).toBeTruthy();
  });
});
