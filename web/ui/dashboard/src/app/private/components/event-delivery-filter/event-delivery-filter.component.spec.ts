import { ComponentFixture, TestBed } from '@angular/core/testing';

import { EventDeliveryFilterComponent } from './event-delivery-filter.component';

describe('EventDeliveryFilterComponent', () => {
  let component: EventDeliveryFilterComponent;
  let fixture: ComponentFixture<EventDeliveryFilterComponent>;

  beforeEach(async () => {
    await TestBed.configureTestingModule({
      imports: [ EventDeliveryFilterComponent ]
    })
    .compileComponents();

    fixture = TestBed.createComponent(EventDeliveryFilterComponent);
    component = fixture.componentInstance;
    fixture.detectChanges();
  });

  it('should create', () => {
    expect(component).toBeTruthy();
  });
});
