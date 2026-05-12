import { ComponentFixture, TestBed } from '@angular/core/testing';

import { EventDeliveryComponent } from './event-delivery.component';
import { RouterTestingModule } from '@angular/router/testing';

describe('EventDeliveryComponent', () => {
  let component: EventDeliveryComponent;
  let fixture: ComponentFixture<EventDeliveryComponent>;

  beforeEach(async () => {
    await TestBed.configureTestingModule({
      imports: [ RouterTestingModule, EventDeliveryComponent]
    })
    .compileComponents();

    fixture = TestBed.createComponent(EventDeliveryComponent);
    component = fixture.componentInstance;
    fixture.detectChanges();
  });

  it('should create', () => {
    expect(component).toBeTruthy();
  });
});
