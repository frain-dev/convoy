import { ComponentFixture, TestBed } from '@angular/core/testing';

import { CreateSubscriptionComponent } from './create-subscription.component';
import { RouterTestingModule } from '@angular/router/testing';

describe('CreateSubscriptionComponent', () => {
  let component: CreateSubscriptionComponent;
  let fixture: ComponentFixture<CreateSubscriptionComponent>;

  beforeEach(async () => {
    await TestBed.configureTestingModule({
      imports: [ RouterTestingModule, CreateSubscriptionComponent ]
    })
    .compileComponents();
  });

  beforeEach(() => {
    fixture = TestBed.createComponent(CreateSubscriptionComponent);
    component = fixture.componentInstance;
    fixture.detectChanges();
  });

  it('should create', () => {
    expect(component).toBeTruthy();
  });
});
