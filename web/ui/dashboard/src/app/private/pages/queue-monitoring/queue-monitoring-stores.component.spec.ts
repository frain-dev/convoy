import { ComponentFixture, TestBed } from '@angular/core/testing';
import { AdminService } from '../admin/admin.service';
import { QueueMonitoringStoresComponent, QueueStore } from './queue-monitoring-stores.component';

function previous(visible = true): QueueStore {
	return { id: 'old', provider: 'redis', role: 'previous', connection: 'connected', visible, visibility_reasons: visible ? ['remaining_work'] : [], actions: [], snapshot: {
		observed_at: '2026-09-22T12:00:00Z',
		queues: [{ name: 'event', pending: 3, processing: 0, scheduled: 4, retry: 5, future: 0, aggregating: 2, archived: 1, completed: 0, unknown: 0, paused: true, oldest_due_age_ms: 100 }]
	} };
}

describe('QueueMonitoringStoresComponent', () => {
	let fixture: ComponentFixture<QueueMonitoringStoresComponent>;
	let getQueueStores: jasmine.Spy;
	beforeEach(async () => {
		getQueueStores = jasmine.createSpy().and.returnValue(new Promise(() => {}));
		await TestBed.configureTestingModule({ imports: [QueueMonitoringStoresComponent], providers: [{ provide: AdminService, useValue: { getQueueStores } }] }).compileComponents();
		fixture = TestBed.createComponent(QueueMonitoringStoresComponent);
	});
	afterEach(() => fixture.destroy());

	it('shows a compact completed notice only while the drained store is freshly known empty', async () => {
		const empty = {...previous(), visibility_reasons: ['operation_drained']};
		empty.snapshot = {...empty.snapshot!, queues: []};
		getQueueStores.and.resolveTo({data: [empty]});
		fixture.detectChanges(); await fixture.whenStable(); fixture.detectChanges();
		expect(fixture.nativeElement.textContent).toContain('Drain verified');
		expect(fixture.nativeElement.querySelector('table')).toBeNull();
		getQueueStores.and.resolveTo({data: [{...empty, connection: 'unknown', visibility_reasons: ['inspection_failed']}]});
		await fixture.componentInstance.refresh(); fixture.detectChanges();
		expect(fixture.nativeElement.textContent).not.toContain('Drain verified');
		expect(fixture.nativeElement.textContent).toContain('Stale observation');
	});

	it('does not certify a completed operation when its fresh inventory contains work', async () => {
		getQueueStores.and.resolveTo({data: [{...previous(), visibility_reasons: ['operation_drained']}]});
		fixture.detectChanges(); await fixture.whenStable(); fixture.detectChanges();
		expect(fixture.nativeElement.textContent).not.toContain('Drain verified');
		expect(fixture.nativeElement.querySelector('table')).not.toBeNull();
	});

	it('hides previous queues while initially loading', () => {
		fixture.detectChanges();
		expect(fixture.nativeElement.querySelector('section')).toBeNull();
	});

	it('shows backend-selected work, preserves stale counts, and hides a confirmed empty store', async () => {
		const component = fixture.componentInstance;
		getQueueStores.and.resolveTo({ data: [previous()] });
		fixture.detectChanges(); await fixture.whenStable(); fixture.detectChanges();
		expect(fixture.nativeElement.textContent).toContain('Previous Redis queue');
		expect(fixture.nativeElement.textContent).toContain('(paused)');
		getQueueStores.and.resolveTo({ data: [{ ...previous(), connection: 'unknown', snapshot: null, visibility_reasons: ['inspection_failed'] }] });
		await component.refresh(); fixture.detectChanges();
		expect(fixture.nativeElement.textContent).toContain('Stale observation');
		expect(component.previous[0].snapshot?.queues[0].pending).toBe(3);
		getQueueStores.and.rejectWith(new Error('offline'));
		await component.refresh(); fixture.detectChanges();
		expect(fixture.nativeElement.textContent).toContain('Previous Redis queue');
		getQueueStores.and.resolveTo({ data: [previous(false)] });
		await component.refresh(); fixture.detectChanges();
		expect(fixture.nativeElement.querySelector('section')).toBeNull();
		getQueueStores.and.rejectWith(new Error('offline after empty read'));
		await component.refresh(); fixture.detectChanges();
		expect(fixture.nativeElement.textContent).toContain('Previous Redis queue');
	});

	it('shows unknown on a first failed backend inspection without inventing counts', async () => {
		getQueueStores.and.resolveTo({ data: [{ ...previous(), connection: 'unknown', snapshot: null }] });
		fixture.detectChanges(); await fixture.whenStable(); fixture.detectChanges();
		expect(fixture.nativeElement.textContent).toContain('remaining work is unknown');
		expect(fixture.nativeElement.querySelector('table')).toBeNull();
	});

	it('does not carry counts to a different store or accept a late response after destruction', async () => {
		const component = fixture.componentInstance;
		getQueueStores.and.resolveTo({ data: [previous()] });
		fixture.detectChanges(); await fixture.whenStable(); fixture.detectChanges();
		getQueueStores.and.resolveTo({ data: [{ ...previous(), id: 'replacement', connection: 'unknown', snapshot: null }] });
		await component.refresh();
		expect(component.previous[0].snapshot).toBeNull();
		let resolve!: (value: unknown) => void;
		getQueueStores.and.returnValue(new Promise(r => resolve = r));
		const pending = component.refresh(); component.ngOnDestroy(); resolve({ data: [] }); await pending;
		expect(component.previous.length).toBe(1);
	});
});
