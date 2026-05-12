import { ComponentFixture, TestBed } from '@angular/core/testing';

import { CreatePortalTransformFunctionComponent } from './create-portal-transform-function.component';
import { RouterTestingModule } from '@angular/router/testing';

describe('CreateTransformFunctionComponent', () => {
  let component: CreatePortalTransformFunctionComponent;
  let fixture: ComponentFixture<CreatePortalTransformFunctionComponent>;

  beforeEach(async () => {
    await TestBed.configureTestingModule({
      imports: [ RouterTestingModule, CreatePortalTransformFunctionComponent]
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
