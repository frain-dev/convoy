import { ComponentFixture, TestBed } from '@angular/core/testing';

import { EventDeliveriesComponent } from './event-deliveries.component';
import { RouterTestingModule } from '@angular/router/testing';

describe('EventDeliveriesComponent', () => {
  let component: EventDeliveriesComponent;
  let fixture: ComponentFixture<EventDeliveriesComponent>;

  beforeEach(async () => {
    await TestBed.configureTestingModule({
      imports: [ RouterTestingModule, EventDeliveriesComponent]
    })
    .compileComponents();

    fixture = TestBed.createComponent(EventDeliveriesComponent);
    component = fixture.componentInstance;
    fixture.detectChanges();
  });

  it('should create', () => {
    expect(component).toBeTruthy();
  });
});
