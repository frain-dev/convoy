import { ComponentFixture, TestBed } from '@angular/core/testing';

import { EndpointsComponent } from './endpoints.component';
import { RouterTestingModule } from '@angular/router/testing';

describe('EndpointsComponent', () => {
  let component: EndpointsComponent;
  let fixture: ComponentFixture<EndpointsComponent>;

  beforeEach(async () => {
    await TestBed.configureTestingModule({
      imports: [ RouterTestingModule, EndpointsComponent]
    })
    .compileComponents();

    fixture = TestBed.createComponent(EndpointsComponent);
    component = fixture.componentInstance;
    fixture.detectChanges();
  });

  it('should create', () => {
    expect(component).toBeTruthy();
  });
});
