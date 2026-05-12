import { ComponentFixture, TestBed } from '@angular/core/testing';

import { ProfileSettingsComponent } from './profile-settings.component';
import { RouterTestingModule } from '@angular/router/testing';

describe('ProfileSettingsComponent', () => {
  let component: ProfileSettingsComponent;
  let fixture: ComponentFixture<ProfileSettingsComponent>;

  beforeEach(async () => {
    await TestBed.configureTestingModule({
      imports: [ RouterTestingModule, ProfileSettingsComponent ]
    })
    .compileComponents();

    fixture = TestBed.createComponent(ProfileSettingsComponent);
    component = fixture.componentInstance;
    fixture.detectChanges();
  });

  it('should create', () => {
    expect(component).toBeTruthy();
  });
});
