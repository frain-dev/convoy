import { ComponentFixture, TestBed } from '@angular/core/testing';

import { EventDeliveryDetailsComponent } from './event-delivery-details.component';
import { RouterTestingModule } from '@angular/router/testing';

describe('EventDeliveryDetailsComponent', () => {
  let component: EventDeliveryDetailsComponent;
  let fixture: ComponentFixture<EventDeliveryDetailsComponent>;

  beforeEach(async () => {
    await TestBed.configureTestingModule({
      imports: [ RouterTestingModule, EventDeliveryDetailsComponent ]
    })
    .compileComponents();
  });

  beforeEach(() => {
    fixture = TestBed.createComponent(EventDeliveryDetailsComponent);
    component = fixture.componentInstance;
    fixture.detectChanges();
  });

  it('should create', () => {
    expect(component).toBeTruthy();
  });
});
