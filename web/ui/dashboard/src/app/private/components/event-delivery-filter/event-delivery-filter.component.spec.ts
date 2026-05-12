import { ComponentFixture, TestBed } from '@angular/core/testing';

import { EventDeliveryFilterComponent } from './event-delivery-filter.component';
import { RouterTestingModule } from '@angular/router/testing';

describe('EventDeliveryFilterComponent', () => {
  let component: EventDeliveryFilterComponent;
  let fixture: ComponentFixture<EventDeliveryFilterComponent>;

  beforeEach(async () => {
    await TestBed.configureTestingModule({
      imports: [ RouterTestingModule, EventDeliveryFilterComponent]
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
