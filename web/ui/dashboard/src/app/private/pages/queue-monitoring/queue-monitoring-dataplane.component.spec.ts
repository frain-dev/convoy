import { CommonModule } from '@angular/common';
import { ComponentFixture, TestBed } from '@angular/core/testing';

import { TagComponent } from 'src/app/components/tag/tag.component';
import { AdminService } from 'src/app/private/pages/admin/admin.service';

import { QueueMonitoringDataplaneComponent } from './queue-monitoring-dataplane.component';

function replica(overrides: any = {}) {
	return {
		replica: 'agent-1',
		mode: 'example',
		running: true,
		sampled_at: '2026-09-01T08:00:00Z',
		age_seconds: 2,
		stale: false,
		stages: [{ name: 'ingest', queued: 4, waiting: 0, workers: 8, partitions: 2, partition_capacity: 10000, deepest_partition: 3 }],
		writers: [{ name: 'events', pending: 12, failures: 0 }],
		counters: [{ name: 'dropped_items', value: 7 }],
		gauges: [{ name: 'endpoints', value: 40 }],
		outstanding: [{ name: 'events_pending', count: 84088, known: true, as_of: '2026-09-01T08:00:00Z' }],
		...overrides
	};
}

function status(replicas: any[], staleAfterSeconds = 15) {
	return { data: { replicas, stale_after_seconds: staleAfterSeconds } };
}

// The service returns the transport error rather than a message for this call,
// because the status code is what tells "no plane here" apart from a failed read.
function httpError(code: number) {
	return { response: { status: code } };
}

describe('QueueMonitoringDataplaneComponent', () => {
	let fixture: ComponentFixture<QueueMonitoringDataplaneComponent>;
	let component: QueueMonitoringDataplaneComponent;
	let reads: Array<() => Promise<any>>;
	let calls: number;

	beforeEach(async () => {
		reads = [];
		calls = 0;

		await TestBed.configureTestingModule({
			imports: [CommonModule, TagComponent, QueueMonitoringDataplaneComponent],
			providers: [
				{
					provide: AdminService,
					useValue: {
						getDataPlaneStatus: () => {
							calls++;
							const next = reads.shift();
							// A read this test did not script is a question it did
							// not set up, so leave it pending rather than answer
							// with the previous response again.
							return next ? next() : new Promise(() => {});
						}
					}
				}
			]
		}).compileComponents();

		fixture = TestBed.createComponent(QueueMonitoringDataplaneComponent);
		component = fixture.componentInstance;

		// The first change detection runs ngOnInit, which reads. Doing it here,
		// before any read is scripted, leaves that read pending forever, so every
		// scripted read below belongs to a load the test asked for. Tests about
		// polling call ngOnInit themselves.
		fixture.detectChanges();
		calls = 0;
	});

	afterEach(() => {
		// The panel polls. Left running, its timer fires into a destroyed fixture
		// and the next test reads state this one wrote.
		fixture.destroy();
	});

	// The load is awaited directly rather than through ngOnInit so the assertion
	// runs against a settled response.
	async function render() {
		await component.load();
		paint();
	}

	// The component's own detector, not fixture.detectChanges: these tests move
	// state between passes on purpose, which is exactly what the fixture's
	// verification pass exists to reject.
	function paint() {
		fixture.changeDetectorRef.detectChanges();
	}

	function text(): string {
		return (fixture.nativeElement as HTMLElement).textContent ?? '';
	}

	it('reports a backlog it could not read as unknown, not as zero', async () => {
		reads.push(() =>
			Promise.resolve(
				status([
					replica({
						outstanding: [
							{ name: 'events_pending', count: 0, known: false, as_of: '2026-09-01T08:00:00Z' },
							{ name: 'deliveries_retry', count: 12, known: true, as_of: '2026-09-01T08:00:00Z' }
						]
					})
				])
			)
		);

		await render();

		expect(text()).toContain('unknown');
		expect(text()).toContain('12');
	});

	it('renders the depth, the fullest lane and the durable backlog', async () => {
		reads.push(() => Promise.resolve(status([replica()])));

		await render();

		expect(text()).toContain('84088');
		expect(text()).toContain('3 / 10000');
		expect(text()).toContain('agent-1');
	});

	it('renders nothing when the deployment wires no data plane monitoring', async () => {
		reads.push(() => Promise.reject(httpError(501)));

		await render();

		expect(component.state).toBe('hidden');
		expect(text().trim()).toBe('');
	});

	it('stops polling once the server says there is no data plane', async () => {
		reads.push(() => Promise.reject(httpError(501)));

		await component.ngOnInit();

		expect(component.state).toBe('hidden');
		expect(component.polling).toBeFalse();
	});

	// ngOnInit awaits the first read before it schedules, so leaving the view
	// during that read lands the schedule after ngOnDestroy. A timer started
	// then is never cleared: the panel keeps polling for the life of the tab,
	// against a component nothing is rendering.
	it('does not start polling when the view is destroyed during the first read', async () => {
		let answer!: (value: any) => void;
		reads.push(() => new Promise(resolve => (answer = resolve)));

		const init = component.ngOnInit();
		fixture.destroy();
		answer(status([replica()]));
		await init;

		expect(component.polling).toBeFalse();
	});

	it('keeps polling after a failed read, which may be transient', async () => {
		reads.push(() => Promise.reject(httpError(500)));

		await component.ngOnInit();

		expect(component.state).toBe('unknown');
		expect(component.polling).toBeTrue();
	});

	// Second pass against state the first wrote: a plane that answers once and
	// then reports itself gone must leave the panel, not keep its last depth.
	it('leaves the panel when a later read says the plane is gone', async () => {
		reads.push(() => Promise.resolve(status([replica()])));
		reads.push(() => Promise.reject(httpError(501)));

		await component.ngOnInit();
		expect(component.polling).toBeTrue();

		await component.load();
		paint();

		expect(component.state).toBe('hidden');
		expect(component.polling).toBeFalse();
		expect(text().trim()).toBe('');
	});

	it('drops the numbers it can no longer vouch for when a refresh fails', async () => {
		reads.push(() => Promise.resolve(status([replica()])));
		reads.push(() => Promise.reject(httpError(500)));

		await render();
		expect(text()).toContain('84088');

		await component.load();
		paint();

		expect(component.state).toBe('unknown');
		// Dropped, not merely hidden: a later render must not be able to bring
		// the previous depth back as though it described the present.
		expect(component.replicas).toEqual([]);
		expect(text()).not.toContain('84088');
		expect(text()).toContain('unknown');
	});

	// The first paint happens before any read has landed, and most instances
	// turn out to have no plane at all. A header and a spinner there would flash
	// a panel onto the page and then take it away again on every visit.
	//
	// beforeEach left the first read pending, which is exactly that moment.
	it('renders nothing before the first read lands', () => {
		paint();

		expect(component.state).toBe('loading');
		expect(text().trim()).toBe('');
	});

	// Nothing on main publishes a snapshot, so a stock instance reads an empty
	// list forever. It must not grow a permanent panel saying so, and it must
	// keep polling, or a plane that starts publishing later needs a reload to
	// appear.
	it('renders nothing while no replica is reporting, and keeps polling', async () => {
		reads.push(() => Promise.resolve(status([])));

		await component.ngOnInit();
		paint();

		expect(component.state).toBe('empty');
		expect(text().trim()).toBe('');
		expect(component.polling).toBeTrue();
	});

	it('shows the panel once a replica starts publishing', async () => {
		reads.push(() => Promise.resolve(status([])));
		reads.push(() => Promise.resolve(status([replica()])));

		await render();
		expect(text().trim()).toBe('');

		await component.load();
		paint();

		expect(component.state).toBe('ready');
		expect(text()).toContain('agent-1');
	});

	it('marks a replica whose last sample is stale', async () => {
		reads.push(() => Promise.resolve(status([replica({ stale: true, age_seconds: 240 })])));

		await render();

		expect(text()).toContain('Stale');
		expect(text()).toContain('are not current');
	});

	// Running false means the live gauges describe nothing. The publisher already
	// clears them, and the panel must not depend on that: a producer that sends
	// depth alongside running false is exactly the case where rendering it as
	// current is the whole failure this panel exists to prevent.
	it('drops the live sections of a replica that is up but not accepting', async () => {
		reads.push(() =>
			Promise.resolve(
				status([
					replica({
						running: false,
						stages: [{ name: 'ingest', queued: 9999, waiting: 3, workers: 4, partitions: 2, partition_capacity: 10000, deepest_partition: 9999 }],
						writers: [{ name: 'events', pending: 42, batches: 7, flush_failures: 0 }],
						gauges: [{ name: 'cached_projects', value: 12 }]
					})
				])
			)
		);

		await render();

		expect(text()).toContain('Not accepting');
		expect(text()).not.toContain('Fullest lane');
		expect(text()).not.toContain('9999');
		expect(text()).not.toContain('cached_projects');
		expect(component.replicas[0].stages).toEqual([]);
		expect(component.replicas[0].writers).toEqual([]);
		expect(component.replicas[0].gauges).toEqual([]);
	});

	// Counters and the durable backlog are not run-scoped: a total still
	// describes the run that ended, and rows outstanding in the database are
	// outstanding whether or not anything is draining them.
	it('keeps counters and outstanding on a replica that is not accepting', async () => {
		reads.push(() =>
			Promise.resolve(
				status([
					replica({
						running: false,
						counters: [{ name: 'recovery_claimed', value: 780 }],
						outstanding: [{ name: 'events_pending', count: 84088, known: true, as_of: '2026-09-01T08:00:00Z' }]
					})
				])
			)
		);

		await render();

		expect(text()).toContain('780');
		expect(text()).toContain('84088');
	});

	it('reads zero workers as a mode, not as an absence', () => {
		const stage = { name: 'deliver', queued: 0, waiting: 0, partitions: 1, partition_capacity: 1, deepest_partition: 0 };

		expect(component.workersLabel({ ...stage, workers: 0 } as any)).toBe('per item');
		expect(component.workersLabel({ ...stage, workers: 8 } as any)).toBe('8');
	});

	// The panel polls, so a slow read can land after a later one. Without the
	// request token the first response would reinstate depth the newer read had
	// already replaced.
	it('ignores a response that a newer read has already superseded', async () => {
		let releaseSlow: ((value: any) => void) | undefined;
		reads.push(() => new Promise(resolve => (releaseSlow = resolve)));
		reads.push(() => Promise.resolve(status([replica({ replica: 'agent-2' })])));

		const slow = component.load();
		const fresh = component.load();
		await fresh;

		releaseSlow?.(status([replica({ replica: 'agent-1' })]));
		await slow;

		expect(calls).toBe(2);
		expect(component.replicas.map(r => r.replica)).toEqual(['agent-2']);
	});
});
