import { ComponentFixture, TestBed } from '@angular/core/testing';

import { CreateSubscriptionFilterComponent } from './create-subscription-filter.component';

describe('CreateSubscriptionFilterComponent', () => {
  let component: CreateSubscriptionFilterComponent;
  let fixture: ComponentFixture<CreateSubscriptionFilterComponent>;

  beforeEach(async () => {
    await TestBed.configureTestingModule({
      imports: [ CreateSubscriptionFilterComponent ]
    })
    .compileComponents();

    fixture = TestBed.createComponent(CreateSubscriptionFilterComponent);
    component = fixture.componentInstance;
    fixture.detectChanges();
  });

  it('should create', () => {
    expect(component).toBeTruthy();
  });
});
