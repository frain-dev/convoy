import { ComponentFixture, TestBed } from '@angular/core/testing';

import { MultiInputComponent } from './multi-input.component';
import { RouterTestingModule } from '@angular/router/testing';

describe('MultiInputComponent', () => {
  let component: MultiInputComponent;
  let fixture: ComponentFixture<MultiInputComponent>;

  beforeEach(async () => {
    await TestBed.configureTestingModule({
      imports: [ RouterTestingModule, MultiInputComponent]
    })
    .compileComponents();

    fixture = TestBed.createComponent(MultiInputComponent);
    component = fixture.componentInstance;
    fixture.detectChanges();
  });

  it('should create', () => {
    expect(component).toBeTruthy();
  });
});
