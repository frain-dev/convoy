import { ComponentFixture, TestBed } from '@angular/core/testing';

import { EndpointsComponent } from './endpoints.component';

describe('EndpointsComponent', () => {
  let component: EndpointsComponent;
  let fixture: ComponentFixture<EndpointsComponent>;

  beforeEach(async () => {
    await TestBed.configureTestingModule({
      imports: [ EndpointsComponent ]
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
