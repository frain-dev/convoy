import { ComponentFixture, TestBed } from '@angular/core/testing';
import { ReactiveFormsModule } from '@angular/forms';
import { RouterTestingModule } from '@angular/router/testing';
import { RadioComponent } from 'src/app/components/radio/radio.component';

import { AddAnalyticsComponent } from './add-analytics.component';

describe('AddAnalyticsComponent', () => {
  let component: AddAnalyticsComponent;
  let fixture: ComponentFixture<AddAnalyticsComponent>;

  beforeEach(async () => {
    await TestBed.configureTestingModule({
      declarations: [ AddAnalyticsComponent ],
      imports: [ReactiveFormsModule, RouterTestingModule, RadioComponent]
    })
    .compileComponents();
  });

  beforeEach(() => {
    fixture = TestBed.createComponent(AddAnalyticsComponent);
    component = fixture.componentInstance;
    fixture.detectChanges();
  });

  it('should create', () => {
    expect(component).toBeTruthy();
  });
});
