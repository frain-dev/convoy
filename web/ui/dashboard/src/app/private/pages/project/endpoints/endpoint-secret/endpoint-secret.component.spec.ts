import { ComponentFixture, TestBed } from '@angular/core/testing';

import { EndpointSecretComponent } from './endpoint-secret.component';

describe('EndpointSecretComponent', () => {
  let component: EndpointSecretComponent;
  let fixture: ComponentFixture<EndpointSecretComponent>;

  beforeEach(async () => {
    await TestBed.configureTestingModule({
      imports: [ EndpointSecretComponent ]
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
