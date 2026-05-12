import { ComponentFixture, TestBed } from '@angular/core/testing';

import { SubscriptionsComponent } from './subscriptions.component';
import { RouterTestingModule } from '@angular/router/testing';

describe('SubscriptionsComponent', () => {
  let component: SubscriptionsComponent;
  let fixture: ComponentFixture<SubscriptionsComponent>;

  beforeEach(async () => {
    await TestBed.configureTestingModule({
      imports: [ RouterTestingModule, SubscriptionsComponent]
    })
    .compileComponents();

    fixture = TestBed.createComponent(SubscriptionsComponent);
    component = fixture.componentInstance;
    fixture.detectChanges();
  });

  it('should create', () => {
    expect(component).toBeTruthy();
  });
});
