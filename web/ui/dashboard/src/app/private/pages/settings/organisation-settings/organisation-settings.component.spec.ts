import { ComponentFixture, TestBed } from '@angular/core/testing';

import { OrganisationSettingsComponent } from './organisation-settings.component';
import { RouterTestingModule } from '@angular/router/testing';

describe('OrganisationSettingsComponent', () => {
  let component: OrganisationSettingsComponent;
  let fixture: ComponentFixture<OrganisationSettingsComponent>;

  beforeEach(async () => {
    await TestBed.configureTestingModule({
      imports: [ RouterTestingModule, OrganisationSettingsComponent]
    })
    .compileComponents();

    fixture = TestBed.createComponent(OrganisationSettingsComponent);
    component = fixture.componentInstance;
    fixture.detectChanges();
  });

  it('should create', () => {
    expect(component).toBeTruthy();
  });
});
