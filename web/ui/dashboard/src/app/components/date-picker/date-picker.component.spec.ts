import { ComponentFixture, TestBed } from '@angular/core/testing';

import { DatePickerComponent } from './date-picker.component';
import { RouterTestingModule } from '@angular/router/testing';

describe('DatePickerComponent', () => {
  let component: DatePickerComponent;
  let fixture: ComponentFixture<DatePickerComponent>;

  beforeEach(async () => {
    await TestBed.configureTestingModule({
      imports: [ RouterTestingModule, DatePickerComponent]
    })
    .compileComponents();

    fixture = TestBed.createComponent(DatePickerComponent);
    component = fixture.componentInstance;
    fixture.detectChanges();
  });

  it('should create', () => {
    expect(component).toBeTruthy();
  });

  it('restores dates and times when the parent range changes without shifting the emitted UTC range', () => {
    const range = { startDate: '2026-09-01T10:15:00', endDate: '2026-09-02T12:30:59' };
    fixture.componentRef.setInput('dateRangeValue', range);
    fixture.detectChanges();
    const start = new Date(range.startDate + 'Z');
    expect(component.selectedStartDay).toBe(new Date(start.getFullYear(), start.getMonth(), start.getDate()).getTime());
    expect(component.filterStartHour).toBe(start.getHours());
    expect(component.filterStartMinute).toBe(15);
    spyOn(component.selectedDateRange, 'emit');
    component.applyDate(true);
    expect(component.selectedDateRange.emit).toHaveBeenCalledWith(range);
  });
});
