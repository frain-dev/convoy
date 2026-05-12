import { ComponentFixture, TestBed } from '@angular/core/testing';

import { EndpointSecretComponent } from './endpoint-secret.component';
import { RouterTestingModule } from '@angular/router/testing';

describe('EndpointSecretComponent', () => {
  let component: EndpointSecretComponent;
  let fixture: ComponentFixture<EndpointSecretComponent>;

  beforeEach(async () => {
    await TestBed.configureTestingModule({
      imports: [ RouterTestingModule, EndpointSecretComponent]
    })
    .compileComponents();

    fixture = TestBed.createComponent(EndpointSecretComponent);
    component = fixture.componentInstance;
    fixture.detectChanges();
  });

  it('should create', () => {
    expect(component).toBeTruthy();
  });
});
