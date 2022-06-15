import { ComponentFixture, TestBed } from '@angular/core/testing';

import { SendEventComponent } from './send-event.component';

describe('SendEventComponent', () => {
  let component: SendEventComponent;
  let fixture: ComponentFixture<SendEventComponent>;

  beforeEach(async () => {
    await TestBed.configureTestingModule({
      declarations: [ SendEventComponent ]
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
