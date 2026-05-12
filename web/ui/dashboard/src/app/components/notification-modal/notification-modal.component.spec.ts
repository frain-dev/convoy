import { ComponentFixture, TestBed } from '@angular/core/testing';

import { NotificationModalComponent } from './notification-modal.component';
import { RouterTestingModule } from '@angular/router/testing';

describe('NotificationModalComponent', () => {
  let component: NotificationModalComponent;
  let fixture: ComponentFixture<NotificationModalComponent>;

  beforeEach(async () => {
    await TestBed.configureTestingModule({
      imports: [ RouterTestingModule, NotificationModalComponent]
    })
    .compileComponents();

    fixture = TestBed.createComponent(NotificationModalComponent);
    component = fixture.componentInstance;
    fixture.detectChanges();
  });

  it('should create', () => {
    expect(component).toBeTruthy();
  });
});
