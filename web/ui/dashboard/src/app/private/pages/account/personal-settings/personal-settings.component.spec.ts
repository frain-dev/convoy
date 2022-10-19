import { ComponentFixture, TestBed } from '@angular/core/testing';

import { PersonalSettingsComponent } from './personal-settings.component';

describe('PersonalSettingsComponent', () => {
  let component: PersonalSettingsComponent;
  let fixture: ComponentFixture<PersonalSettingsComponent>;

  beforeEach(async () => {
    await TestBed.configureTestingModule({
      declarations: [ PersonalSettingsComponent ]
    })
    .compileComponents();

    fixture = TestBed.createComponent(PersonalSettingsComponent);
    component = fixture.componentInstance;
    fixture.detectChanges();
  });

  it('should create', () => {
    expect(component).toBeTruthy();
  });
});
