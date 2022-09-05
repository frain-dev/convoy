import { TestBed } from '@angular/core/testing';
import { RouterTestingModule } from '@angular/router/testing';

import { CreateAppService } from './create-app.service';

describe('CreateAppService', () => {
  let service: CreateAppService;

  beforeEach(() => {
    TestBed.configureTestingModule({
        imports: [RouterTestingModule]
    });
    service = TestBed.inject(CreateAppService);
  });

  it('should be created', () => {
    expect(service).toBeTruthy();
  });
});
