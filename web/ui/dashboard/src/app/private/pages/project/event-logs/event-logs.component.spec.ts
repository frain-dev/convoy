import { ComponentFixture, TestBed } from '@angular/core/testing';

import { EventLogsComponent } from './event-logs.component';
import { RouterTestingModule } from '@angular/router/testing';

describe('EventLogsComponent', () => {
  let component: EventLogsComponent;
  let fixture: ComponentFixture<EventLogsComponent>;

  beforeEach(async () => {
    await TestBed.configureTestingModule({
      imports: [ RouterTestingModule, EventLogsComponent]
    })
    .compileComponents();

    fixture = TestBed.createComponent(EventLogsComponent);
    component = fixture.componentInstance;
    fixture.detectChanges();
  });

  it('should create', () => {
    expect(component).toBeTruthy();
  });
});

describe('EventLogsComponent route chips', () => {
	const logs = Object.create(EventLogsComponent.prototype) as EventLogsComponent;

	it('labels a subscription match as Matched, not Delivered', () => {
		const matched = { endpoints: [{ uid: 'ep_1' }] } as any;
		const unmatched = { endpoints: [] } as any;

		expect(logs.getRouteStatus(matched)).toBe('Matched');
		expect(logs.getRouteStatus(unmatched)).toBe('Unmatched');
		expect(logs.routeChipClass(matched)).toBe('bg-success-a3 text-success-11');
		expect(logs.routeChipClass(unmatched)).toBe('bg-new.surface-muted text-new.text-secondary');
	});
});
