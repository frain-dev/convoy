import { ComponentFixture, TestBed } from '@angular/core/testing';

import { EventDeliveryComponent } from './event-delivery.component';

describe('EventDeliveryComponent', () => {
  let component: EventDeliveryComponent;
  let fixture: ComponentFixture<EventDeliveryComponent>;

  beforeEach(async () => {
    await TestBed.configureTestingModule({
      declarations: [ EventDeliveryComponent ]
    })
    .compileComponents();
  });

  beforeEach(() => {
    fixture = TestBed.createComponent(EventDeliveryComponent);
    component = fixture.componentInstance;
    fixture.detectChanges();
  });

  it('should create', () => {
    expect(component).toBeTruthy();
  });
});
