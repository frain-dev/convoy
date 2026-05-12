import { ComponentFixture, TestBed } from '@angular/core/testing';

import { TimePickerComponent } from './time-picker.component';
import { RouterTestingModule } from '@angular/router/testing';

describe('TimePickerComponent', () => {
  let component: TimePickerComponent;
  let fixture: ComponentFixture<TimePickerComponent>;

  beforeEach(async () => {
    await TestBed.configureTestingModule({
      imports: [ RouterTestingModule, TimePickerComponent]
    })
    .compileComponents();

    fixture = TestBed.createComponent(TimePickerComponent);
    component = fixture.componentInstance;
    fixture.detectChanges();
  });

  it('should create', () => {
    expect(component).toBeTruthy();
  });
});
