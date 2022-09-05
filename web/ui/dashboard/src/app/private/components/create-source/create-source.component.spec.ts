import { ComponentFixture, TestBed } from '@angular/core/testing';
import { ReactiveFormsModule } from '@angular/forms';
import { RouterTestingModule } from '@angular/router/testing';
import { InputComponent } from 'src/app/components/input/input.component';
import { RadioComponent } from 'src/app/components/radio/radio.component';

import { CreateSourceComponent } from './create-source.component';

describe('CreateSourceComponent', () => {
  let component: CreateSourceComponent;
  let fixture: ComponentFixture<CreateSourceComponent>;

  beforeEach(async () => {
    await TestBed.configureTestingModule({
      declarations: [ CreateSourceComponent ],
      imports:[ReactiveFormsModule, RouterTestingModule, InputComponent, RadioComponent]
    })
    .compileComponents();
  });

  beforeEach(() => {
    fixture = TestBed.createComponent(CreateSourceComponent);
    component = fixture.componentInstance;
    fixture.detectChanges();
  });

  it('should create', () => {
    expect(component).toBeTruthy();
  });
});
