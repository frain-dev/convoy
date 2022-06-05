import { ComponentFixture, TestBed } from '@angular/core/testing';

import { EventDeliveriesComponent } from './event-deliveries.component';

describe('EventDeliveriesComponent', () => {
  let component: EventDeliveriesComponent;
  let fixture: ComponentFixture<EventDeliveriesComponent>;

  beforeEach(async () => {
    await TestBed.configureTestingModule({
      declarations: [ EventDeliveriesComponent ]
    })
    .compileComponents();
  });

  beforeEach(() => {
    fixture = TestBed.createComponent(EventDeliveriesComponent);
    component = fixture.componentInstance;
    fixture.detectChanges();
  });

  it('should create', () => {
    expect(component).toBeTruthy();
  });
});
