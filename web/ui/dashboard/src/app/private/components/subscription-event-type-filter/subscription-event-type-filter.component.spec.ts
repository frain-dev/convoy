import { ComponentFixture, TestBed } from '@angular/core/testing';

import { SubscriptionEventTypeFilterComponent } from './subscription-event-type-filter.component';

describe('SubscriptionEventTypeFilterComponent', () => {
	let component: SubscriptionEventTypeFilterComponent;
	let fixture: ComponentFixture<SubscriptionEventTypeFilterComponent>;

	beforeEach(async () => {
		await TestBed.configureTestingModule({
			declarations: [SubscriptionEventTypeFilterComponent]
		}).compileComponents();

		fixture = TestBed.createComponent(SubscriptionEventTypeFilterComponent);
		component = fixture.componentInstance;
		fixture.detectChanges();
	});

	it('should create', () => {
		expect(component).toBeTruthy();
	});
});
