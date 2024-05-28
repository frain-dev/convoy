import { ComponentFixture, TestBed } from '@angular/core/testing';

import { CreateTransformFunctionComponent } from './create-transform-function.component';

describe('CreateTransformFunctionComponent', () => {
  let component: CreateTransformFunctionComponent;
  let fixture: ComponentFixture<CreateTransformFunctionComponent>;

  beforeEach(async () => {
    await TestBed.configureTestingModule({
      imports: [ CreateTransformFunctionComponent ]
    })
    .compileComponents();

    fixture = TestBed.createComponent(CreateTransformFunctionComponent);
    component = fixture.componentInstance;
    fixture.detectChanges();
  });

  it('should create', () => {
    expect(component).toBeTruthy();
  });
});
