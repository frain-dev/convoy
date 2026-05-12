import { ComponentFixture, TestBed } from '@angular/core/testing';

import { SecuritySettingsComponent } from './security-settings.component';
import { RouterTestingModule } from '@angular/router/testing';

describe('SecuritySettingsComponent', () => {
  let component: SecuritySettingsComponent;
  let fixture: ComponentFixture<SecuritySettingsComponent>;

  beforeEach(async () => {
    await TestBed.configureTestingModule({
      imports: [ RouterTestingModule, SecuritySettingsComponent ]
    })
    .compileComponents();

    fixture = TestBed.createComponent(SecuritySettingsComponent);
    component = fixture.componentInstance;
    fixture.detectChanges();
  });

  it('should create', () => {
    expect(component).toBeTruthy();
  });
});
