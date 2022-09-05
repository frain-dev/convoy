import { ComponentFixture, TestBed } from '@angular/core/testing';
import { ReactiveFormsModule } from '@angular/forms';
import { RouterTestingModule } from '@angular/router/testing';
import { InputComponent } from 'src/app/components/input/input.component';
import { SelectComponent } from 'src/app/components/select/select.component';

import { SendEventComponent } from './send-event.component';

describe('SendEventComponent', () => {
  let component: SendEventComponent;
  let fixture: ComponentFixture<SendEventComponent>;

  beforeEach(async () => {
    await TestBed.configureTestingModule({
      declarations: [ SendEventComponent ],
      imports:[ReactiveFormsModule, RouterTestingModule, InputComponent, SelectComponent]
    })
    .compileComponents();
  });

  beforeEach(() => {
    fixture = TestBed.createComponent(SendEventComponent);
    component = fixture.componentInstance;
    fixture.detectChanges();
  });

  it('should create', () => {
    expect(component).toBeTruthy();
  });
});
