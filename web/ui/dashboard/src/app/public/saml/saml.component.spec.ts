import { ComponentFixture, TestBed } from '@angular/core/testing';

import { SamlComponent } from './saml.component';
import { RouterTestingModule } from '@angular/router/testing';

describe('SamlComponent', () => {
  let component: SamlComponent;
  let fixture: ComponentFixture<SamlComponent>;

  beforeEach(async () => {
    await TestBed.configureTestingModule({
      imports: [ RouterTestingModule, SamlComponent]
    })
    .compileComponents();

    fixture = TestBed.createComponent(SamlComponent);
    component = fixture.componentInstance;
    fixture.detectChanges();
  });

  it('should create', () => {
    expect(component).toBeTruthy();
  });
});
