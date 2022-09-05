import { TestBed } from '@angular/core/testing';
import { ReactiveFormsModule } from '@angular/forms';
import { RouterTestingModule } from '@angular/router/testing';

import { CreateSourceService } from './create-source.service';

describe('CreateSourceService', () => {
  let service: CreateSourceService;

  beforeEach(() => {
    TestBed.configureTestingModule({
        imports: [ReactiveFormsModule, RouterTestingModule]
    });
    service = TestBed.inject(CreateSourceService);
  });

  it('should be created', () => {
    expect(service).toBeTruthy();
  });
});
