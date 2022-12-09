import { ComponentFixture, TestBed } from '@angular/core/testing';

import { EndpointDetailsComponent } from './endpoint-details.component';

describe('EndpointDetailsComponent', () => {
  let component: EndpointDetailsComponent;
  let fixture: ComponentFixture<EndpointDetailsComponent>;

  beforeEach(async () => {
    await TestBed.configureTestingModule({
      imports: [ EndpointDetailsComponent ]
    })
    .compileComponents();

    fixture = TestBed.createComponent(EndpointDetailsComponent);
    component = fixture.componentInstance;
    fixture.detectChanges();
  });

  it('should create', () => {
    expect(component).toBeTruthy();
  });
});
