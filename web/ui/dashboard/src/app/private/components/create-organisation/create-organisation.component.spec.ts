import { ComponentFixture, TestBed } from '@angular/core/testing';

import { CreateOrganisationComponent } from './create-organisation.component';

describe('CreateOrganisationComponent', () => {
  let component: CreateOrganisationComponent;
  let fixture: ComponentFixture<CreateOrganisationComponent>;

  beforeEach(async () => {
    await TestBed.configureTestingModule({
      declarations: [ CreateOrganisationComponent ]
    })
    .compileComponents();
  });

  beforeEach(() => {
    fixture = TestBed.createComponent(CreateOrganisationComponent);
    component = fixture.componentInstance;
    fixture.detectChanges();
  });

  it('should create', () => {
    expect(component).toBeTruthy();
  });
});
