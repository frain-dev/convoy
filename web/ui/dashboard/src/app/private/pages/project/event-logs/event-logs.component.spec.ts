import { ComponentFixture, TestBed } from '@angular/core/testing';

import { EventLogsComponent } from './event-logs.component';
import { RouterTestingModule } from '@angular/router/testing';

describe('EventLogsComponent', () => {
  let component: EventLogsComponent;
  let fixture: ComponentFixture<EventLogsComponent>;

  beforeEach(async () => {
    await TestBed.configureTestingModule({
      imports: [ RouterTestingModule, EventLogsComponent]
    })
    .compileComponents();

    fixture = TestBed.createComponent(EventLogsComponent);
    component = fixture.componentInstance;
    fixture.detectChanges();
  });

  it('should create', () => {
    expect(component).toBeTruthy();
  });
});
