import { ComponentFixture, TestBed } from '@angular/core/testing';

import { CreateSourceComponent } from './create-source.component';
import { RouterTestingModule } from '@angular/router/testing';

describe('CreateSourceComponent', () => {
  let component: CreateSourceComponent;
  let fixture: ComponentFixture<CreateSourceComponent>;

  beforeEach(async () => {
    await TestBed.configureTestingModule({
      imports: [ RouterTestingModule, CreateSourceComponent ]
    })
    .compileComponents();
  });

  beforeEach(() => {
    fixture = TestBed.createComponent(CreateSourceComponent);
    component = fixture.componentInstance;
    fixture.detectChanges();
  });

  it('should create', () => {
    expect(component).toBeTruthy();
  });
});
