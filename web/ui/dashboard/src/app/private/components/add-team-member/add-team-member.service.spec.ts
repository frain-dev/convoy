import { TestBed } from '@angular/core/testing';

import { AddTeamMemberService } from './add-team-member.service';

describe('AddTeamMemberService', () => {
  let service: AddTeamMemberService;

  beforeEach(() => {
    TestBed.configureTestingModule({});
    service = TestBed.inject(AddTeamMemberService);
  });

  it('should be created', () => {
    expect(service).toBeTruthy();
  });
});
