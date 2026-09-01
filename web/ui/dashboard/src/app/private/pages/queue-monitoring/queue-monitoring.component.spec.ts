import { CommonModule } from '@angular/common';
import { ComponentFixture, TestBed } from '@angular/core/testing';
import { ActivatedRoute, Router } from '@angular/router';

import { AdminService } from 'src/app/private/pages/admin/admin.service';
import { LicensesService } from 'src/app/services/licenses/licenses.service';
import { RbacService } from 'src/app/services/rbac/rbac.service';

import { QueueMonitoringComponent, QueueMonitoringSegment } from './queue-monitoring.component';

const UNLICENSED_COPY = 'Queue monitoring requires a license key';

describe('QueueMonitoringComponent', () => {
	let fixture: ComponentFixture<QueueMonitoringComponent>;
	let licensed: boolean;
	let role: string;
	let answerRole!: () => void;
	let answerLicenses!: () => void;

	beforeEach(async () => {
		licensed = false;
		role = 'INSTANCE_ADMIN';

		await TestBed.configureTestingModule({
			imports: [CommonModule, QueueMonitoringComponent],
			providers: [
				// Left pending. The surface and data plane reads only happen on a
				// licensed instance, and neither one is what these tests are about.
				{
					provide: AdminService,
					useValue: {
						getQueueStats: () => new Promise(() => {}),
						getDataPlaneStatus: () => new Promise(() => {})
					}
				},
				{
					provide: LicensesService,
					useValue: {
						// Both reads ngOnInit awaits are held open so a test can paint
						// the page at the moment each one is still in flight. That
						// moment is the whole subject here: it is the state every
						// visit to the tab passes through.
						loadAllLicenses: () => new Promise<void>(resolve => (answerLicenses = resolve as () => void)),
						hasInstanceLicense: () => licensed
					}
				},
				{ provide: RbacService, useValue: { getUserRole: () => new Promise(resolve => (answerRole = () => resolve(role))) } },
				{ provide: Router, useValue: { navigateByUrl: () => {}, navigate: () => Promise.resolve(true) } },
				// The segmented control's selection lives in the URL, so the page
				// reads the landing query params. No segment is chosen in these
				// tests; they are about the license gate.
				{ provide: ActivatedRoute, useValue: { snapshot: { queryParams: {} } } }
			]
		}).compileComponents();

		fixture = TestBed.createComponent(QueueMonitoringComponent);

		// The first pass runs ngOnInit, so ngOnInit is never called by hand here:
		// a second call would start a second pair of reads and overwrite the
		// resolvers the test is holding.
		fixture.detectChanges();
	});

	afterEach(() => {
		fixture.destroy();
	});

	// The component's own detector, not fixture.detectChanges: these tests move
	// state between passes on purpose, which is what the fixture's verification
	// pass exists to reject.
	function paint() {
		fixture.changeDetectorRef.detectChanges();
	}

	function text(): string {
		return (fixture.nativeElement as HTMLElement).textContent ?? '';
	}

	// The only loader that can reach the page here is the one under test: the
	// surface read is left pending, so the asynqmon child (which carries the
	// other loader in this folder) is never rendered.
	function loader(): Element | null {
		return (fixture.nativeElement as HTMLElement).querySelector('convoy-loader');
	}

	// A resolved promise continues on the microtask queue, so the awaits inside
	// ngOnInit need a few turns to reach the next read.
	async function settle() {
		for (let turn = 0; turn < 5; turn++) await Promise.resolve();
	}

	it('does not claim the instance is unlicensed before the role read lands', () => {
		paint();

		expect(text()).not.toContain(UNLICENSED_COPY);
	});

	it('does not claim the instance is unlicensed while the license read is in flight', async () => {
		answerRole();
		await settle();

		paint();

		expect(text()).not.toContain(UNLICENSED_COPY);
	});

	it('claims it once the license read answers that the instance is unlicensed', async () => {
		answerRole();
		await settle();
		answerLicenses();
		await settle();

		paint();

		expect(text()).toContain(UNLICENSED_COPY);
	});

	it('never claims it on a licensed instance', async () => {
		licensed = true;

		answerRole();
		await settle();
		answerLicenses();
		await settle();

		paint();

		expect(text()).not.toContain(UNLICENSED_COPY);
	});

	it('shows a loader on the first pass, before the role read lands', () => {
		paint();

		expect(loader()).not.toBeNull();
	});

	it('keeps the loader while the license read is in flight', async () => {
		answerRole();
		await settle();

		paint();

		expect(loader()).not.toBeNull();
	});

	it('drops the loader once the license read answers that the instance is unlicensed', async () => {
		answerRole();
		await settle();
		answerLicenses();
		await settle();

		paint();

		expect(loader()).toBeNull();
		expect(text()).toContain(UNLICENSED_COPY);
	});

	it('drops the loader once the license read answers that the instance is licensed', async () => {
		licensed = true;

		answerRole();
		await settle();
		answerLicenses();
		await settle();

		paint();

		expect(loader()).toBeNull();
	});

	// A non-admin is navigated away, so no verdict is ever read here. The loader
	// still has to go: a navigation that does not complete would otherwise leave
	// it spinning forever, and the banner must stay away because nothing was read.
	it('drops the loader on the non-admin path without claiming the instance is unlicensed', async () => {
		role = 'PROJECT_VIEWER';

		answerRole();
		await settle();

		paint();

		expect(loader()).toBeNull();
		expect(text()).not.toContain(UNLICENSED_COPY);
	});
});

// The segmented control is the placement decision: one tab, two engines, one on
// screen at a time, and never a mode switch that hides an engine that is running.
// When a data plane runs both are live, so what this chooses is which half is
// being read, not which half exists.
describe('QueueMonitoringComponent segments', () => {
	const DATA_PLANE: QueueMonitoringSegment = 'dataplane';
	const QUEUE: QueueMonitoringSegment = 'queue';

	let fixture: ComponentFixture<QueueMonitoringComponent>;
	let component: QueueMonitoringComponent;
	let planeReplicas: any[];
	let queryParams: Record<string, string>;
	let navigations: any[];

	function replica(overrides: any = {}) {
		return {
			replica: 'agent-1',
			mode: 'example',
			running: true,
			sampled_at: '2026-09-01T08:00:00Z',
			age_seconds: 2,
			stale: false,
			stages: [],
			writers: [],
			counters: [],
			gauges: [],
			outstanding: [{ name: 'events_pending', count: 5, known: true, as_of: '2026-09-01T08:00:00Z' }],
			...overrides
		};
	}

	async function start() {
		await TestBed.configureTestingModule({
			imports: [CommonModule, QueueMonitoringComponent],
			providers: [
				{
					provide: AdminService,
					useValue: {
						// Left pending, so the surface stays 'loading' and neither
						// queue surface mounts. Which queue surface renders is not
						// what these tests are about, and the asynqmon child brings
						// its own dependencies with it.
						getQueueStats: () => new Promise(() => {}),
						getDataPlaneStatus: () => Promise.resolve({ data: { replicas: planeReplicas, stale_after_seconds: 15 } })
					}
				},
				{ provide: LicensesService, useValue: { loadAllLicenses: () => Promise.resolve(), hasInstanceLicense: () => true } },
				{ provide: RbacService, useValue: { getUserRole: () => Promise.resolve('INSTANCE_ADMIN') } },
				{
					provide: Router,
					useValue: {
						navigateByUrl: () => {},
						navigate: (_commands: any[], options: any) => {
							navigations.push(options);
							return Promise.resolve(true);
						}
					}
				},
				{ provide: ActivatedRoute, useValue: { snapshot: { queryParams } } }
			]
		}).compileComponents();

		fixture = TestBed.createComponent(QueueMonitoringComponent);
		component = fixture.componentInstance;

		// The page awaits a role read and a license read before the panel is even
		// created, and the panel then awaits its own first status read before it
		// can report. Each pass here is one of those hops.
		fixture.detectChanges();
		for (let pass = 0; pass < 4; pass++) {
			await settle();
			fixture.changeDetectorRef.detectChanges();
		}
	}

	beforeEach(() => {
		TestBed.resetTestingModule();
		planeReplicas = [replica()];
		queryParams = {};
		navigations = [];
	});

	afterEach(() => {
		fixture?.destroy();
	});

	// Resolved promises continue on the microtask queue, and the page awaits a
	// role read, a license read and the panel's first status read before the
	// segment can be decided.
	async function settle() {
		for (let turn = 0; turn < 20; turn++) await Promise.resolve();
	}

	function segmentButtons(): HTMLButtonElement[] {
		return Array.from((fixture.nativeElement as HTMLElement).querySelectorAll('button[aria-pressed]'));
	}

	function click(label: string) {
		const button = segmentButtons().find(candidate => candidate.textContent?.trim() === label);
		button?.click();
		fixture.changeDetectorRef.detectChanges();
	}

	// Ownership of HTTP events is read from what the plane published, because the
	// dashboard has no flag for it. A replica accepting work is the engine those
	// events go through.
	it('lands on the data plane when a replica is accepting work', async () => {
		await start();

		expect(component.planeReports).toBeTrue();
		expect(component.segment).toBe(DATA_PLANE);
		expect(segmentButtons().length).toBe(2);
	});

	// A plane that reports and is accepting nothing has taken no HTTP events, so
	// the queue is the engine carrying them and the queue is the landing view.
	it('lands on the queue when the plane reports and nothing is accepting', async () => {
		planeReplicas = [replica({ running: false })];

		await start();

		expect(component.planeReports).toBeTrue();
		expect(component.segment).toBe(QUEUE);
		expect(segmentButtons().length).toBe(2);
	});

	// A queue-only instance sees the tab exactly as it did before: no control, and
	// nothing above the queue view pushing it down.
	it('shows no control at all when no plane reports', async () => {
		planeReplicas = [];

		await start();

		expect(component.planeReports).toBeFalse();
		expect(component.segment).toBe(QUEUE);
		expect(segmentButtons().length).toBe(0);
	});

	describe('URL round trip', () => {
		it('lands on the segment the shared link names, against the default rule', async () => {
			queryParams = { engine: 'queue' };

			await start();

			// The default rule would have chosen the data plane here.
			expect(component.segment).toBe(QUEUE);
		});

		it('honours a link naming the data plane once the plane reports', async () => {
			queryParams = { engine: 'dataplane' };
			planeReplicas = [replica({ running: false })];

			await start();

			expect(component.segment).toBe(DATA_PLANE);
		});

		// A link naming a segment that does not exist on this instance must not
		// leave a blank page: the derived getter falls back to the only view there
		// is.
		it('falls back to the queue when a link names the data plane and none reports', async () => {
			queryParams = { engine: 'dataplane' };
			planeReplicas = [];

			await start();

			expect(component.segment).toBe(QUEUE);
			expect(segmentButtons().length).toBe(0);
		});

		it('ignores a value it does not recognise and applies the default rule', async () => {
			queryParams = { engine: 'both' };

			await start();

			expect(component.segment).toBe(DATA_PLANE);
		});

		// An explicit value on every write, merged into the params already there,
		// and replacing rather than stacking history. A null clear would leave the
		// previous segment in the URL for the next reader of it.
		it('writes the chosen segment to the URL', async () => {
			await start();

			click('Queue');

			expect(component.segment).toBe(QUEUE);
			expect(navigations.length).toBe(1);
			expect(navigations[0].queryParams).toEqual({ engine: 'queue' });
			expect(navigations[0].queryParamsHandling).toBe('merge');
			expect(navigations[0].replaceUrl).toBeTrue();
		});

		it('writes nothing when the segment already on screen is clicked again', async () => {
			await start();

			click('Data plane');

			expect(navigations.length).toBe(0);
		});

		it('round trips back to the data plane', async () => {
			await start();

			click('Queue');
			click('Data plane');

			expect(component.segment).toBe(DATA_PLANE);
			expect(navigations.map(navigation => navigation.queryParams)).toEqual([{ engine: 'queue' }, { engine: 'dataplane' }]);
		});
	});

	// The panel polls, so the report arrives again every few seconds. Re-deciding
	// the default on each one would move the view under whoever is reading it.
	it('does not move the segment when a later report changes which engine is accepting', async () => {
		await start();
		expect(component.segment).toBe(DATA_PLANE);

		component.onPlaneReported({ reports: true, known: true, replicas: 1, accepting: 0 });
		fixture.changeDetectorRef.detectChanges();

		expect(component.segment).toBe(DATA_PLANE);
	});

	it('keeps an operator on the segment they chose when a later report disagrees', async () => {
		await start();

		click('Queue');
		component.onPlaneReported({ reports: true, known: true, replicas: 1, accepting: 1 });
		fixture.changeDetectorRef.detectChanges();

		expect(component.segment).toBe(QUEUE);
	});

	// The control disappearing must take the selection with it, or the page shows
	// neither engine.
	it('returns to the queue when the plane stops reporting', async () => {
		await start();
		expect(component.segment).toBe(DATA_PLANE);

		component.onPlaneReported({ reports: false, known: true, replicas: 0, accepting: 0 });
		fixture.changeDetectorRef.detectChanges();

		expect(component.segment).toBe(QUEUE);
		expect(segmentButtons().length).toBe(0);
	});

	// Each view carries one line about the other engine, because when a plane runs
	// both are live and the answer may be on the other side.
	it('names the other engine on each side', async () => {
		await start();

		expect(component.otherEngineLine).toContain('The queue still carries');

		click('Queue');

		expect(component.otherEngineLine).toContain('1 data plane replica reporting');
		expect(component.otherEngineLine).toContain('1 accepting work');
	});

	// A blipped poll keeps the segment, because the plane is still there. What it
	// must not do is turn that line into a count of zero replicas.
	it('does not count replicas on the line when the plane read failed', async () => {
		await start();

		click('Queue');
		component.onPlaneReported({ reports: true, known: false, replicas: 0, accepting: 0 });
		fixture.changeDetectorRef.detectChanges();

		expect(component.segment).toBe(QUEUE);
		expect(component.otherEngineLine).toContain('how many replicas are up is unknown');
		expect(component.otherEngineLine).not.toContain('0 data plane');
	});
});
