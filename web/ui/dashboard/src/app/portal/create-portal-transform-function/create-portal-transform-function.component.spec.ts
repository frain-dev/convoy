import { ComponentFixture, TestBed } from '@angular/core/testing';

import { CreatePortalTransformFunctionComponent } from './create-portal-transform-function.component';

describe('CreateTransformFunctionComponent', () => {
  let component: CreatePortalTransformFunctionComponent;
  let fixture: ComponentFixture<CreatePortalTransformFunctionComponent>;

  beforeEach(async () => {
    await TestBed.configureTestingModule({
      imports: [ CreatePortalTransformFunctionComponent ]
    })
    .compileComponents();

    fixture = TestBed.createComponent(CreatePortalTransformFunctionComponent);
    component = fixture.componentInstance;
    fixture.detectChanges();
  });

  it('should create', () => {
    expect(component).toBeTruthy();
  });
});
