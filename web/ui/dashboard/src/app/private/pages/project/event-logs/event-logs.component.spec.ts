import { ComponentFixture, TestBed } from '@angular/core/testing';

import { EventLogsComponent } from './event-logs.component';

describe('EventLogsComponent', () => {
  let component: EventLogsComponent;
  let fixture: ComponentFixture<EventLogsComponent>;

  beforeEach(async () => {
    await TestBed.configureTestingModule({
      imports: [ EventLogsComponent ]
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
