import { CommonModule } from '@angular/common';
import { ComponentFixture, TestBed } from '@angular/core/testing';

import { EngineVerdictComponent } from 'src/app/components/engine-verdict/engine-verdict.component';
import { TagComponent } from 'src/app/components/tag/tag.component';
import { AdminService } from 'src/app/private/pages/admin/admin.service';

import { DataPlaneReport, QueueMonitoringDataplaneComponent } from './queue-monitoring-dataplane.component';

// Default gauges carry no throughput, which is the plane's first sample after it
// starts: the publisher has no earlier reading to difference and omits the four
// interval gauges together.
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

// A measured interval, as gauges, with the names spelled exactly as the
// persisted snapshot spells them. There is no throughput section on the wire:
// throughput arrives as entries in the same flat gauges array as everything else,
// and a fixture that invented a nested shape would pass against a reader looking
// for the same invention, which is the bug this shape exists to catch.
//
// 4954ms because the publisher reports the real elapsed time between its two
// readings rather than its configured 5s period.
function measured(overrides: { window_ms?: number; accepted?: number; delivered?: number; failed?: number } = {}) {
	const window = { window_ms: 4954, accepted: 0, delivered: 0, failed: 0, ...overrides };

	return [
		{ name: 'throughput_window_ms', value: window.window_ms },
		{ name: 'throughput_accepted', value: window.accepted },
		{ name: 'throughput_delivered', value: window.delivered },
		{ name: 'throughput_failed', value: window.failed }
	];
}

// The restart sample: the restart gauge alone, with the four interval gauges
// absent, because the interval that spanned the restart was discarded rather
// than reported. So a restart is also an unmeasured sample.
function restarted() {
	return [{ name: 'throughput_restarted', value: 1 }];
}

// The retry backlog's schedule, as gauges, spelled as the persisted snapshot
// spells them. Both or neither: an age with no due time says nothing about
// whether the backlog is draining, and a due time with no age says nothing about
// how long it has not been.
//
// Signed on purpose. A negative due time is a retry that was due that long ago.
function retrySchedule(oldestAgeMs: number, nextDueInMs: number) {
	return [
		{ name: 'outstanding_oldest_retry_age_ms', value: oldestAgeMs },
		{ name: 'outstanding_next_retry_due_in_ms', value: nextDueInMs }
	];
}

function retries(count: number, known = true) {
	return [{ name: 'deliveries_retry', count, known, as_of: '2026-09-01T08:00:00Z' }];
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
	let reports: DataPlaneReport[];

	beforeEach(async () => {
		reads = [];
		calls = 0;
		reports = [];

		await TestBed.configureTestingModule({
			imports: [CommonModule, TagComponent, EngineVerdictComponent, QueueMonitoringDataplaneComponent],
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
		component.reported.subscribe(report => reports.push(report));

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

	function openDiagnostics() {
		component.toggleDiagnostics('agent-1');
		paint();
	}

	// Every absent value on the panel, as the two kinds of reader reach it: the
	// glyph in the value slot, and the label a screen reader or a tablet gets,
	// since the tooltip body only appears on hover and focus.
	function absentValues(): Array<{ glyph: string; label: string }> {
		return Array.from((fixture.nativeElement as HTMLElement).querySelectorAll<HTMLElement>('convoy-tooltip')).map(tooltip => ({
			glyph: (tooltip.querySelector<HTMLElement>('[tooltipToggle]')?.textContent ?? '').trim(),
			label: tooltip.querySelector('button')?.getAttribute('aria-label') ?? ''
		}));
	}

	// Every tooltip on the panel as a keyboard or screen reader user reaches it.
	function tooltipLabels(): string[] {
		return Array.from((fixture.nativeElement as HTMLElement).querySelectorAll<HTMLElement>('convoy-tooltip button')).map(button => button.getAttribute('aria-label') ?? '');
	}

	// A plane on its first sample: no interval to report yet. Both causes of an
	// absent interval are benign and both are knowable from the sample, so the
	// panel names the cause rather than reporting an unexplained absence, and it
	// must not fill the gap with a zero or look empty while doing it.
	describe('a plane with no measured interval', () => {
		it('names the reason rather than reporting an unexplained absence', async () => {
			reads.push(() => Promise.resolve(status([replica()])));

			await render();

			expect(text()).toContain('Measuring. No earlier sample from this engine to compare against, so there is no interval yet. Next sample carries one. 84,088 outstanding.');
			expect(text()).not.toContain('not reported');
			expect(text()).not.toContain('Idle');
			expect(text()).not.toContain('Nothing accepted');
		});

		// A dash an operator cannot account for reads as a broken panel, so the
		// tooltip and the accessible label both carry the cause.
		it('renders the two interval numbers as a dash carrying that reason', async () => {
			reads.push(() => Promise.resolve(status([replica()])));

			await render();

			const absent = absentValues().filter(value => value.glyph === '-');
			expect(absent.length).toBe(2);
			expect(absent[0].label).toContain('Events in: No earlier sample yet.');
			expect(absent[0].label).toContain('A number appears on the next sample. Nothing is wrong.');
			expect(absent[1].label).toContain('Deliveries out: No earlier sample yet.');
		});

		// A benign absence is not a fault, so the leading dot must not paint one.
		it('does not paint a benign absence as a problem', async () => {
			reads.push(() => Promise.resolve(status([replica()])));

			await render();

			const dot = (fixture.nativeElement as HTMLElement).querySelector('convoy-engine-verdict span');
			expect(dot?.className).toContain('bg-new.text-secondary');
			expect(dot?.className).not.toContain('bg-warning-9');
		});

		// The durable backlog is real data the plane does publish, so the panel is
		// not empty in this state even though two of the three numbers are not.
		it('still shows the outstanding backlog it does have', async () => {
			reads.push(() => Promise.resolve(status([replica()])));

			await render();

			expect(text()).toContain('Outstanding');
			expect(text()).toContain('84,088');
		});

		// A plane that reported no backlog section has not reported an empty one.
		it('reports an unread backlog as unknown, not as zero', async () => {
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

			// The dash, and the reason on the accessible label, for the same
			// reason the interval numbers use it: an unread count printed as 0
			// reads as an empty backlog.
			// Selected by the dash rather than by the label, because the label
			// tooltip explaining what Outstanding means is on the same metric and
			// carries the same prefix.
			const dashes = absentValues().filter(value => value.glyph === '-');
			const outstanding = dashes.find(value => value.label.startsWith('Outstanding:'));
			expect(outstanding?.label).toContain('Not read.');
			expect(outstanding?.label).toContain('unknown rather than zero');
			expect(text()).toContain('Outstanding could not be read');
			expect(text()).not.toContain('nothing outstanding');

			openDiagnostics();
			expect(text()).toContain('12');
		});

		it('reports an absent backlog section as unknown too', async () => {
			reads.push(() => Promise.resolve(status([replica({ outstanding: [] })])));

			await render();

			expect(text()).toContain('Outstanding could not be read');
			expect(text()).not.toContain('Idle');
		});
	});

	describe('a plane that measured an interval', () => {
		// The regression the panel shipped with: the verdict and the numbers read
		// one shape while the diagnostics table iterated the gauges the plane
		// actually sends, so the card reported "not reported" above its own live
		// throughput. The gauge names here are written out rather than built by
		// the helper, because a helper renamed alongside the reader would hide
		// exactly this.
		it('reads the interval the plane published rather than reporting it absent', async () => {
			reads.push(() =>
				Promise.resolve(
					status([
						replica({
							gauges: [
								{ name: 'throughput_window_ms', value: 4994 },
								{ name: 'throughput_accepted', value: 288 },
								{ name: 'throughput_delivered', value: 287 },
								{ name: 'throughput_failed', value: 30 },
								{ name: 'reference_projects', value: 1 }
							],
							outstanding: [{ name: 'deliveries_retry', count: 583, known: true, as_of: '2026-09-01T08:00:00Z' }]
						})
					])
				)
			);

			await render();

			expect(text()).toContain('Draining. 288 accepted, 287 completed and 30 failed or discarded in the last ~5s, 583 outstanding.');
			expect(text()).toContain('288');
			expect(text()).toContain('287');
			expect(text()).not.toContain('not reported');
			expect(absentValues().filter(value => value.glyph === '-').length).toBe(0);
		});

		// The same fields at zero with a valid window, which is the idle plane
		// thirteen seconds later. A dash here would make an idle plane look
		// broken, which is the same error in the opposite direction.
		it('renders a measured zero as zero rather than as a dash', async () => {
			reads.push(() =>
				Promise.resolve(
					status([
						replica({
							gauges: [
								{ name: 'throughput_window_ms', value: 4997 },
								{ name: 'throughput_accepted', value: 0 },
								{ name: 'throughput_delivered', value: 0 },
								{ name: 'throughput_failed', value: 0 }
							],
							outstanding: [{ name: 'events_pending', count: 0, known: true, as_of: '2026-09-01T08:00:00Z' }]
						})
					])
				)
			);

			await render();

			expect(text()).toContain('Running and idle. It measured the last ~5s and accepted nothing, nothing outstanding.');
			expect(absentValues().filter(value => value.glyph === '-').length).toBe(0);

			const values = Array.from((fixture.nativeElement as HTMLElement).querySelectorAll<HTMLElement>('[data-flow-value]')).map(node => (node.textContent ?? '').trim());
			expect(values).toEqual(['0', '0', '0']);
		});

		// The staging state: a plane that is up, measured an interval and took
		// nothing, on an instance whose events all reach Convoy some other way.
		// A card of zeros cannot be told from a quiet plane, so the zero on the
		// intake says it was measured, and the identity says why this replica
		// might be the only thing that is quiet.
		it('says a measured zero intake was a reading rather than a gap', async () => {
			reads.push(() =>
				Promise.resolve(
					status([
						replica({
							gauges: measured({ accepted: 0, delivered: 0 }),
							outstanding: [{ name: 'events_pending', count: 0, known: true, as_of: '2026-09-01T08:00:00Z' }]
						})
					])
				)
			);

			await render();

			expect(text()).toContain('measured, nothing reached this replica');
			expect(text()).toContain('If work is arriving on this instance, it is not coming through this engine.');

			const scope = tooltipLabels().find(label => label.includes('Replica scope'));
			expect(scope).toContain('This replica only counts work its own process accepted.');
			expect(scope).toContain('agent HTTP port');
			expect(scope).toContain('carried by the queue instead');
		});

		// The note is a reading, so it may only appear where there was one. Beside
		// a dash it would read as an explanation of the absence.
		it('keeps the measured note off an interval it could not measure', async () => {
			reads.push(() => Promise.resolve(status([replica({})])));

			await render();

			expect(text()).not.toContain('measured, nothing reached this replica');
		});

		// Events in and deliveries out count different things, so the sentence
		// must not invite a subtraction that fanout makes meaningless: one event
		// legitimately produces more than one delivery.
		it('states both units and never nets them against each other', async () => {
			reads.push(() =>
				Promise.resolve(
					status([
						replica({
							gauges: measured({ accepted: 3, delivered: 12 }),
							outstanding: [{ name: 'events_pending', count: 4, known: true, as_of: '2026-09-01T08:00:00Z' }]
						})
					])
				)
			);

			await render();

			expect(text()).toContain('Draining. 3 accepted and 12 completed in the last ~5s, 4 outstanding.');
			expect(text()).toContain('Events in');
			expect(text()).toContain('Deliveries out');
			expect(text()).not.toContain('more out than in');
			expect(text()).not.toContain('Backlog growing');
		});

		// The card Smart read as an incident: a backlog of 583 while the window
		// completed nothing, next to a failure total of 1,176. The plane was
		// healthy. Those outstanding items were deliveries scheduled to retry
		// later, and most of the failures belonged to a different, paused
		// endpoint whose deliveries never entered the backlog at all.
		it('does not call a backlog of scheduled retries stuck', async () => {
			reads.push(() =>
				Promise.resolve(
					status([
						replica({
							gauges: [...measured({ accepted: 40, delivered: 0, failed: 0 }), ...retrySchedule(96_000, 42_000)],
							counters: [{ name: 'deliveries_discarded', value: 1176 }],
							outstanding: retries(583)
						})
					])
				)
			);

			await render();

			expect(text()).toContain('40 accepted and nothing completed in the last ~5s, 583 outstanding.');
			expect(text()).toContain('Work waiting on a schedule is outstanding too, so read what that number is made of before taking this as held up.');

			// And the schedule says which it is: an oldest retry a minute and a
			// half old with the next one due shortly is a backlog draining on
			// time, which is the reading the card could not make when this was an
			// incident.
			expect(text()).toContain('oldest retry 1m old');
			expect(text()).toContain('next due in 40s');
			expect(text()).not.toContain('overdue');
			expect(text()).not.toContain('stuck');
			expect(text()).not.toContain('Not draining');

			// And not painted as one either.
			const dot = (fixture.nativeElement as HTMLElement).querySelector('convoy-engine-verdict span');
			expect(dot?.className).not.toContain('bg-warning-9');
		});

		// The window counts a discard the same as an exhausted retry, because it
		// answers whether accepted work left the outstanding set and both of those
		// do. Naming it after the narrower outcome now that the lifetime totals
		// are split would make the other one invisible here.
		it('names the interval count for both terminal outcomes, not just failure', async () => {
			reads.push(() => Promise.resolve(status([replica({ gauges: measured({ accepted: 10, delivered: 8, failed: 2 }) })])));

			await render();

			expect(text()).toContain('Failures reported');
			expect(text()).toContain('2 deliveries failed or discarded in the last ~5s');
			expect(text()).not.toContain('2 deliveries failed in the last');
			expect(text()).not.toContain('Where it is stuck');
		});
	});

	// A restart discards the interval that spanned it, so the plane sends the
	// restart gauge alone. That is an unmeasured sample with a knowable cause.
	describe('a plane that restarted underneath the sampler', () => {
		it('says the restart is why there is no interval, without calling it a fault', async () => {
			reads.push(() => Promise.resolve(status([replica({ gauges: restarted() })])));

			await render();

			expect(text()).toContain('Measuring again. The engine restarted, so the interval spanning the restart was discarded. Next sample carries one. 84,088 outstanding.');
			expect(text()).not.toContain('No earlier sample');

			const dot = (fixture.nativeElement as HTMLElement).querySelector('convoy-engine-verdict span');
			expect(dot?.className).not.toContain('bg-warning-9');
		});

		it('gives the dash a different reason from a plane that has only just started', async () => {
			reads.push(() => Promise.resolve(status([replica({ gauges: restarted() })])));

			await render();

			const absent = absentValues().filter(value => value.glyph === '-');
			expect(absent[0].label).toContain('Events in: Plane restarted.');
			expect(absent[0].label).not.toContain('No earlier sample yet.');
		});
	});

	// The four interval gauges are one compound value. A count without the window
	// it covers is not a rate, so a partial publish is unknown rather than a
	// number with a zero defaulted into the hole. The failure count is non-zero
	// on purpose: a zero would be skipped by the count check alone, and the test
	// would pass with the omitted-together rule removed.
	it('treats a partly published interval as unknown rather than defaulting the rest to zero', async () => {
		reads.push(() => Promise.resolve(status([replica({ writers: [], stages: [], counters: [], gauges: [{ name: 'throughput_failed', value: 3 }] })])));

		await render();

		expect(text()).not.toContain('deliveries failed in the last');
		expect(text()).toContain('Throughput reported incompletely, so whether work is draining is unknown.');
		expect(absentValues().filter(value => value.glyph === '-').length).toBe(2);
	});

	// What the backlog is doing, which is the reading the bare count could not
	// give. The two gauges are one compound value on the same rule as the
	// interval, and the difference between a backlog that is empty and one that
	// could not be read is carried by the flag the count already has rather than
	// by their absence, because they are absent in both cases.
	describe('the retry schedule under the outstanding count', () => {
		it('states the oldest retry and the next due time as two plain readings', async () => {
			reads.push(() => Promise.resolve(status([replica({ gauges: [...measured({ accepted: 12, delivered: 9 }), ...retrySchedule(140_000, 25_000)], outstanding: retries(31) })])));

			await render();

			expect(text()).toContain('oldest retry 2m old');
			expect(text()).toContain('next due in 23s');
			expect(text()).not.toContain('overdue');
			expect(absentValues().filter(value => value.glyph === '-').length).toBe(0);
		});

		// An empty backlog has no oldest retry and nothing due, so the publisher
		// omits both gauges. That is an answer, not a gap: rendering the dash here
		// would report a backlog it could not read when it read an empty one. The
		// publisher does not send zeros for it either, because a zero age already
		// means a retry created this instant.
		it('reads a known empty backlog as empty rather than as unknown', async () => {
			reads.push(() => Promise.resolve(status([replica({ gauges: measured({ accepted: 4, delivered: 4 }), outstanding: retries(0) })])));

			await render();

			expect(text()).toContain('no retries waiting');
			expect(text()).not.toContain('oldest retry');
			expect(absentValues().filter(value => value.glyph === '-').length).toBe(0);

			const values = Array.from((fixture.nativeElement as HTMLElement).querySelectorAll<HTMLElement>('[data-flow-value]')).map(node => (node.textContent ?? '').trim());
			expect(values).toEqual(['4', '4', '0']);
		});

		// The count could not be read, so neither could its schedule. The dash on
		// the count already carries that, and a schedule line beside it would be a
		// reading of rows nobody counted.
		it('says nothing about the schedule of a backlog it could not read', async () => {
			reads.push(() => Promise.resolve(status([replica({ gauges: measured({ accepted: 4, delivered: 4 }), outstanding: retries(0, false) })])));

			await render();

			expect(text()).not.toContain('no retries waiting');
			expect(text()).not.toContain('oldest retry');

			const dashes = absentValues().filter(value => value.glyph === '-');
			expect(dashes.find(value => value.label.startsWith('Outstanding:'))?.label).toContain('Not read.');
		});

		// Overdue is not stuck. A retry can be a little late because the scanner
		// has not reached it, and one sample cannot see whether that is what this
		// is, so the numbers are stated and the extra line only appears past a
		// minute, which is an order of magnitude beyond scan latency and beyond
		// the age of the sample being read.
		it('states an overdue retry as overdue and stops short of calling it stuck', async () => {
			reads.push(() => Promise.resolve(status([replica({ gauges: [...measured({ accepted: 0, delivered: 0 }), ...retrySchedule(3_600_000, -720_000)], outstanding: retries(583) })])));

			await render();

			expect(text()).toContain('oldest retry 1h old');
			expect(text()).toContain('next due 12m ago');
			expect(text()).toContain('overdue by longer than a scan cycle explains');
			expect(text()).not.toContain('stuck');
		});

		// A few seconds late is the scanner working, so it gets the reading and
		// nothing more.
		it('leaves a slightly overdue retry as a reading', async () => {
			reads.push(() => Promise.resolve(status([replica({ gauges: [...measured({ accepted: 6, delivered: 6 }), ...retrySchedule(30_000, -8_000)], outstanding: retries(2) })])));

			await render();

			expect(text()).toContain('next due 10s ago');
			expect(text()).not.toContain('overdue by longer');
		});

		// Both readings are carried forward by the snapshot's own age, from the
		// server that read sampled_at, and never by the browser's clock. Crossing
		// two clocks here would fold their skew into a number an operator pages
		// on, which is the reason the publisher sends an age rather than a
		// timestamp. A reader subtracting the browser's now from sampled_at would
		// report months, this fixture being dated.
		it('ages the schedule by the snapshot, not by the browser clock', async () => {
			reads.push(() => Promise.resolve(status([replica({ age_seconds: 45, gauges: [...measured({ accepted: 1, delivered: 1 }), ...retrySchedule(15_000, 60_000)], outstanding: retries(9) })])));

			await render();

			expect(text()).toContain('oldest retry 1m old');
			expect(text()).toContain('next due in 15s');
		});

		// A total is unknown as soon as any one backlog is, and the retry rows can
		// still have been read. Refusing to state a schedule that was read, over an
		// unread count somewhere else, would drop the most useful thing on the card.
		it('states a schedule it read even when another backlog made the total unknown', async () => {
			reads.push(() =>
				Promise.resolve(
					status([
						replica({
							gauges: [...measured({ accepted: 2, delivered: 2 }), ...retrySchedule(20_000, 10_000)],
							outstanding: [{ name: 'events_pending', count: 0, known: false, as_of: '2026-09-01T08:00:00Z' }, ...retries(6)]
						})
					])
				)
			);

			await render();

			const dashes = absentValues().filter(value => value.glyph === '-');
			expect(dashes.find(value => value.label.startsWith('Outstanding:'))?.label).toContain('Not read.');
			expect(text()).toContain('oldest retry 22s old');
			expect(text()).toContain('next due in 8s');
		});

		// Carrying the reading forward is a display convenience, so it may not
		// become a claim about what happened while nobody was looking. An old
		// sample of a healthy backlog crosses any threshold just by sitting there,
		// and a panel that turned its own staleness into an overdue claim would be
		// the false alarm again with a new cause. So the line is judged on what
		// the plane read, and only the number shown moves.
		it('does not turn its own staleness into an overdue retry', async () => {
			reads.push(() => Promise.resolve(status([replica({ age_seconds: 300, stale: true, gauges: [...measured({ accepted: 2, delivered: 2 }), ...retrySchedule(60_000, 30_000)], outstanding: retries(5) })])));

			await render();

			expect(text()).toContain('next due 4m ago');
			expect(text()).not.toContain('overdue by longer');
		});

		// The contract says a non-zero known count comes with both gauges, since
		// the count and the readings come from one statement over one set of rows.
		// A count without them is a plane that is not running or a bug, so the
		// panel says nothing rather than inventing a schedule, and the count it
		// did read stays a count.
		it('says nothing about a schedule the plane did not publish beside a real count', async () => {
			reads.push(() => Promise.resolve(status([replica({ gauges: measured({ accepted: 2, delivered: 2 }), outstanding: retries(44) })])));

			await render();

			expect(text()).toContain('44');
			expect(text()).not.toContain('oldest retry');
			expect(text()).not.toContain('no retries waiting');
			expect(absentValues().filter(value => value.glyph === '-').length).toBe(0);
		});

		// Consumed above, so the table cannot print the same two numbers again
		// under their raw names beside the sentence that already used them.
		it('keeps the schedule gauges out of diagnostics', async () => {
			reads.push(() => Promise.resolve(status([replica({ gauges: [...measured({ accepted: 2, delivered: 2 }), ...retrySchedule(1_000, 1_000)], outstanding: retries(3) })])));

			await render();
			openDiagnostics();

			expect(text()).not.toContain('Outstanding oldest retry age ms');
			expect(text()).not.toContain('Outstanding next retry due in ms');
		});
	});

	// The section that caused the false alarm. A climbing backlog under a heading
	// reading "Where it is stuck", with a failure total listed beneath it, was
	// read as one phenomenon. It was two: deliveries legitimately waiting on a
	// retry schedule, and terminal discards belonging to a different, paused
	// endpoint. The panel states both as observations now, in separate sections.
	describe('where work is sitting, and failures, as separate facts', () => {
		it('names the place rather than printing a table row', async () => {
			reads.push(() =>
				Promise.resolve(
					status([
						replica({
							stages: [{ name: 'deliver', queued: 512, waiting: 3, workers: 0, partitions: 4, partition_capacity: 10000, deepest_partition: 400 }],
							writers: [{ name: 'event_deliveries', pending: 1200, failures: 4 }],
							counters: []
						})
					])
				)
			);

			await render();

			expect(text()).toContain('Where work is sitting');
			expect(text()).toContain('Deliver: 512 queued, 3 senders waiting');
			expect(text()).toContain('Event deliveries writer: 1,200 pending');
			expect(text()).toContain('4 write batches failed');
			// The heading that asserted a conclusion the data cannot reach.
			expect(text()).not.toContain('Where it is stuck');
		});

		// The failure totals cover every endpoint the plane serves, and a paused
		// endpoint's deliveries are discarded before dispatch without ever
		// joining the backlog. So a failure count sitting under the backlog is an
		// attribution the plane never made.
		it('keeps failures out of the section that lists where work is sitting', async () => {
			reads.push(() =>
				Promise.resolve(
					status([
						replica({
							stages: [{ name: 'deliver', queued: 512, waiting: 0, workers: 4, partitions: 4, partition_capacity: 10000, deepest_partition: 400 }],
							writers: [],
							counters: [{ name: 'delivery_failures', value: 1176 }]
						})
					])
				)
			);

			await render();

			const sections = Array.from((fixture.nativeElement as HTMLElement).querySelectorAll<HTMLElement>('ul')).map(list => (list.previousElementSibling?.textContent ?? '') + '|' + (list.textContent ?? ''));
			const sitting = sections.find(section => section.startsWith('Where work is sitting'));
			const failures = sections.find(section => section.startsWith('Failures reported'));

			expect(sitting).toContain('512 queued');
			expect(sitting).not.toContain('1,176');
			expect(failures).toContain('Delivery failures: 1,176');
			expect(text()).toContain('not attributed to the work outstanding above');
		});

		// Failures and recovery errors are the only counters that always mean
		// something is wrong. A counter whose name carries none of those words
		// stays in diagnostics, because the plane names its own counters and this
		// build cannot have a dictionary of names it has never seen.
		it('promotes a non-zero failure counter and leaves a neutral one behind', async () => {
			reads.push(() =>
				Promise.resolve(
					status([
						replica({
							stages: [],
							writers: [],
							counters: [
								{ name: 'recovery_errors', value: 6 },
								{ name: 'handoff_recovered', value: 900 },
								{ name: 'recovery_errors_previous', value: 0 }
							]
						})
					])
				)
			);

			await render();

			expect(text()).toContain('Recovery errors: 6');
			expect(text()).not.toContain('Handoff recovered: 900');
		});

		// The two lifetime totals are different events and the card said one
		// number. That was half of why it read as an incident: the number was
		// dominated by a paused endpoint's discards, which never joined the
		// backlog beside it, while the backlog belonged to an endpoint retrying
		// normally.
		it('states discarded and failed as the two different outcomes they are', async () => {
			reads.push(() =>
				Promise.resolve(
					status([
						replica({
							stages: [],
							writers: [],
							counters: [
								{ name: 'deliveries_discarded', value: 1176 },
								{ name: 'deliveries_failed', value: 12 }
							]
						})
					])
				)
			);

			await render();

			expect(text()).toContain('1,176 deliveries discarded before any attempt');
			expect(text()).toContain('12 deliveries failed after an attempt');
			expect(text()).toContain('Discarded means the plane never sent it, so it never joined that backlog');

			// And the word rule that promoted the failed total on its name must
			// not print it a second time under its raw label.
			expect(text()).not.toContain('Deliveries failed: 12');
		});

		it('renders neither section when nothing is non-zero', async () => {
			reads.push(() =>
				Promise.resolve(
					status([
						replica({
							stages: [{ name: 'ingest', queued: 0, waiting: 0, workers: 8, partitions: 2, partition_capacity: 10000, deepest_partition: 0 }],
							writers: [{ name: 'events', pending: 0, failures: 0 }],
							counters: [{ name: 'dropped_items', value: 0 }]
						})
					])
				)
			);

			await render();

			expect(text()).not.toContain('Where work is sitting');
			expect(text()).not.toContain('Failures reported');
		});
	});

	describe('labels', () => {
		it('never renders a raw identifier in the primary view', async () => {
			reads.push(() =>
				Promise.resolve(
					status([
						replica({
							stages: [{ name: 'HANDOFF_DELIVER', queued: 5, waiting: 0, workers: 4, partitions: 1, partition_capacity: 10, deepest_partition: 5 }],
							writers: [],
							counters: [{ name: 'RECOVERY_ERRORS', value: 3 }]
						})
					])
				)
			);

			await render();

			expect(text()).toContain('Handoff deliver: 5 queued');
			expect(text()).toContain('Recovery errors: 3');
			expect(text()).not.toContain('HANDOFF_DELIVER');
			expect(text()).not.toContain('RECOVERY_ERRORS');
		});

		it('humanises a name it has never seen and keeps acronyms upper case', () => {
			expect(component.label('applied_lsn')).toBe('Applied LSN');
			expect(component.label('SOME_NEW_THING')).toBe('Some new thing');
			expect(component.label('deliveries-retry')).toBe('Deliveries retry');
			expect(component.label('')).toBe('Unnamed');
		});
	});

	describe('diagnostics', () => {
		it('is collapsed by default and holds the tables that used to be on top', async () => {
			reads.push(() => Promise.resolve(status([replica()])));

			await render();

			expect(component.isOpen('agent-1')).toBeFalse();
			expect(text()).toContain('Show diagnostics');
			expect(text()).not.toContain('Fullest lane');
			expect(text()).not.toContain('3 / 10000');
			expect(text()).not.toContain('Failed batches');
		});

		it('expands to the full stage table, writer table, gauges and counters', async () => {
			reads.push(() => Promise.resolve(status([replica()])));

			await render();
			openDiagnostics();

			expect(component.isOpen('agent-1')).toBeTrue();
			expect(text()).toContain('Hide diagnostics');
			expect(text()).toContain('Fullest lane');
			expect(text()).toContain('3 / 10000');
			expect(text()).toContain('Failed batches');
			expect(text()).toContain('Endpoints');
			expect(text()).toContain('Counters since this replica started');
		});

		it('collapses again on a second click', async () => {
			reads.push(() => Promise.resolve(status([replica()])));

			await render();
			openDiagnostics();
			openDiagnostics();

			expect(component.isOpen('agent-1')).toBeFalse();
			expect(text()).not.toContain('Fullest lane');
		});

		// Replicas come and go. An open flag for one that left must not survive
		// for the life of the tab.
		it('forgets an open replica once it stops reporting', async () => {
			reads.push(() => Promise.resolve(status([replica()])));
			reads.push(() => Promise.resolve(status([replica({ replica: 'agent-2' })])));

			await render();
			openDiagnostics();

			await render();

			expect(component.isOpen('agent-1')).toBeFalse();
		});
	});

	// The plane names its own stages, writers, counters and backlogs. Two sharing
	// one name is a duplicate key, which throws and takes the whole panel down,
	// so every one of these lists is tracked positionally.
	it('renders a plane that reports duplicate item names', async () => {
		reads.push(() =>
			Promise.resolve(
				status([
					replica({
						stages: [
							{ name: 'deliver', queued: 1, waiting: 0, workers: 1, partitions: 1, partition_capacity: 10, deepest_partition: 1 },
							{ name: 'deliver', queued: 2, waiting: 0, workers: 1, partitions: 1, partition_capacity: 10, deepest_partition: 2 }
						],
						writers: [
							{ name: 'events', pending: 3, failures: 0 },
							{ name: 'events', pending: 4, failures: 0 }
						],
						counters: [
							{ name: 'recovery_errors', value: 1 },
							{ name: 'recovery_errors', value: 2 }
						],
						gauges: [
							{ name: 'endpoints', value: 5 },
							{ name: 'endpoints', value: 6 }
						],
						outstanding: [
							{ name: 'events_pending', count: 7, known: true, as_of: '2026-09-01T08:00:00Z' },
							{ name: 'events_pending', count: 8, known: true, as_of: '2026-09-01T08:00:00Z' }
						]
					})
				])
			)
		);

		await render();
		openDiagnostics();

		expect(component.state).toBe('ready');
		// Both duplicates rendered, and the backlog total is their sum rather
		// than whichever one won a key collision.
		expect(text()).toContain('15');
		expect(text()).toContain('Deliver: 1 queued');
		expect(text()).toContain('Deliver: 2 queued');
		expect(text()).toContain('Recovery errors: 1');
		expect(text()).toContain('Recovery errors: 2');
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
		expect(text()).toContain('84,088');

		await component.load();
		paint();

		expect(component.state).toBe('unknown');
		// Dropped, not merely hidden: a later render must not be able to bring
		// the previous depth back as though it described the present.
		expect(component.rows).toEqual([]);
		expect(text()).not.toContain('84,088');
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

	it('renders nothing while another segment is on screen, and keeps polling', async () => {
		component.active = false;
		reads.push(() => Promise.resolve(status([replica()])));

		await component.ngOnInit();
		paint();

		expect(component.state).toBe('ready');
		expect(text().trim()).toBe('');
		expect(component.polling).toBeTrue();
	});

	it('marks a replica whose last sample is stale', async () => {
		reads.push(() => Promise.resolve(status([replica({ stale: true, age_seconds: 240 })])));

		await render();

		expect(text()).toContain('Stale');
		expect(text()).toContain('Not reporting. Last sample 4m ago.');
		expect(text()).toContain('is not current');
	});

	// A stale sample's numbers describe a moment that has passed, so the verdict
	// must not read a rate off them however healthy they looked.
	it('does not read a rate off a stale sample', async () => {
		reads.push(() =>
			Promise.resolve(
				status([
					replica({
						stale: true,
						age_seconds: 240,
						gauges: measured({ accepted: 1240, delivered: 1190 })
					})
				])
			)
		);

		await render();

		expect(text()).toContain('Not reporting. Last sample 4m ago.');
		expect(text()).not.toContain('Draining');
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
						writers: [{ name: 'events', pending: 42, failures: 0 }],
						gauges: [{ name: 'cached_projects', value: 12 }, ...measured({ accepted: 5000, delivered: 4000 })]
					})
				])
			)
		);

		await render();

		expect(text()).toContain('Not accepting');
		// A plane that is not running reads as that, in words, and never as the
		// running plane that measured an interval and found nothing in it.
		expect(text()).toContain('Not running. This engine is not accepting work, so nothing new is entering it.');
		expect(text()).not.toContain('It measured the last');
		expect(text()).not.toContain('9999');
		expect(text()).not.toContain('5,000');
		expect(component.rows[0].replica.stages).toEqual([]);
		expect(component.rows[0].replica.writers).toEqual([]);
		expect(component.rows[0].replica.gauges).toEqual([]);
		// The interval came out of those dropped gauges, so it goes with them, and
		// it goes as an absence rather than as a zero: a stopped plane has not
		// reported an idle interval.
		expect(component.rows[0].flow.measured).toBeUndefined();
	});

	// The gauges of a stopped replica are dropped on the way in, so the accessor
	// sees the same empty list a plane on its first sample publishes and can only
	// call it a first sample. Saying a number arrives next sample promises one
	// from a plane that has stopped taking work, and it says it in the one place
	// a keyboard or screen reader user reads, directly under a verdict saying the
	// opposite.
	it('does not promise a next sample from a replica that stopped accepting', async () => {
		reads.push(() => Promise.resolve(status([replica({ running: false })])));

		await render();
		openDiagnostics();

		const dashes = absentValues().filter(value => value.glyph === '-');
		const intake = dashes.find(value => value.label.startsWith('Events in:'))?.label ?? '';

		expect(intake).toContain('Plane not accepting.');
		expect(intake).not.toContain('No earlier sample yet.');
		expect(dashes.find(value => value.label.startsWith('Deliveries out:'))?.label).toContain('Plane not accepting.');
		// Diagnostics reads the same reason off the row, so a reader who opened it
		// to find out why is not given the other account.
		expect(text()).toContain('Plane not accepting.');
		expect(text()).not.toContain('A number appears on the next sample.');
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

		expect(text()).toContain('84,088');

		openDiagnostics();
		expect(text()).toContain('780');
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
		expect(component.rows.map(row => row.replica.replica)).toEqual(['agent-2']);
	});

	// The page around this panel decides whether the Data plane segment exists at
	// all from these reports, so a failed read must not invent a plane and a
	// plane that answered once must not lose its segment to one.
	describe('what it reports to the page', () => {
		it('reports a plane once a replica has published', async () => {
			reads.push(() => Promise.resolve(status([replica(), replica({ replica: 'agent-2', running: false })])));

			await render();

			expect(reports.pop()).toEqual({ reports: true, known: true, replicas: 2, accepting: 1 });
		});

		it('reports no plane while no replica is publishing', async () => {
			reads.push(() => Promise.resolve(status([])));

			await render();

			expect(reports.pop()).toEqual({ reports: false, known: true, replicas: 0, accepting: 0 });
		});

		it('reports no plane when the server wires none', async () => {
			reads.push(() => Promise.reject(httpError(501)));

			await render();

			expect(reports.pop()).toEqual({ reports: false, known: true, replicas: 0, accepting: 0 });
		});

		// A transport failure is not evidence that a plane exists, so a queue-only
		// instance must not grow a Data plane segment from one.
		it('reports no plane when the very first read fails', async () => {
			reads.push(() => Promise.reject(httpError(500)));

			await render();

			expect(reports.pop()?.reports).toBeFalse();
		});

		// A failure after the plane has answered is the read breaking, not the
		// plane leaving. Dropping the segment there would move the queue view
		// under the operator every time a poll blipped.
		it('keeps reporting a plane when a later read fails', async () => {
			reads.push(() => Promise.resolve(status([replica()])));
			reads.push(() => Promise.reject(httpError(500)));

			await render();
			await render();

			expect(reports.pop()?.reports).toBeTrue();
		});

		// The counts are what the page prints beside the control, so a failed read
		// must mark them as describing nothing. Zero replicas from a read that
		// broke is not a plane that emptied.
		it('marks the counts unknown when a read fails and known when it does not', async () => {
			reads.push(() => Promise.resolve(status([replica()])));
			reads.push(() => Promise.reject(httpError(500)));

			await render();
			expect(reports.pop()?.known).toBeTrue();

			await render();
			expect(reports.pop()).toEqual({ reports: true, known: false, replicas: 0, accepting: 0 });
		});

		// A plane that goes away and then a failed read must not resurrect the
		// segment from the memory of the plane that left.
		it('does not resurrect a plane that left when a later read fails', async () => {
			reads.push(() => Promise.resolve(status([replica()])));
			reads.push(() => Promise.resolve(status([])));
			reads.push(() => Promise.reject(httpError(500)));

			await render();
			await render();
			await render();

			expect(reports.pop()?.reports).toBeFalse();
		});
	});

	// Karma loads src/styles.scss, so Tailwind is really applied and the browser
	// really lays out. The panel is given a 375px frame, the narrowest phone
	// width the dashboard is expected to survive, and asked whether anything
	// pushes past it. A wide table is allowed to scroll inside its own wrapper;
	// what is not allowed is the panel making the page scroll sideways.
	describe('at a 375px viewport', () => {
		it('keeps everything inside the frame, with diagnostics open and long names', async () => {
			const host = fixture.nativeElement as HTMLElement;
			const frame = host.parentElement as HTMLElement;
			frame.style.width = '375px';
			frame.style.overflow = 'hidden';

			reads.push(() =>
				Promise.resolve(
					status([
						replica({
							// Names the plane chose, not names this build knows. One of
							// them carries no separator at all, so it humanises to a
							// single unbreakable word, which is the shape that pushed
							// the old panel off the side of the page.
							replica: 'convoydataplaneagentdeploymentreplicasixfourceninebseven',
							stages: [{ name: 'http_ingest_partitioned_fair_queue_admission', queued: 1234567, waiting: 890123, workers: 0, partitions: 4096, partition_capacity: 1000000, deepest_partition: 999999 }],
							writers: [{ name: 'event_deliveries_bulk_writer', pending: 1234567, failures: 89012 }],
							counters: [
								{ name: 'handoffrecoveryerrorstotalfromacustomplanewithnoseparators', value: 1234567890 },
								{ name: 'deliveries_discarded', value: 1234567890 },
								{ name: 'deliveries_failed', value: 1234567890 }
							],
							// The overdue reading, because it is the longest thing the
							// schedule can put under a number, on the narrowest card.
							gauges: [{ name: 'subscription_cache_entries', value: 1234567 }, ...measured({ accepted: 1234567, delivered: 1234567 }), ...retrySchedule(359_999_000, -720_000)],
							outstanding: [{ name: 'eventdeliveriespendingretrywithnoseparatorsatall', count: 1234567890, known: true, as_of: '2026-09-01T08:00:00Z' }, ...retries(1234567890)]
						})
					])
				)
			);

			await render();
			openDiagnostics();

			expect(frame.clientWidth).toBe(375);
			expect(overflowing(host)).toEqual([]);
		});
	});
});

// A box whose content is wider than the box, and which will neither clip nor
// scroll it, is what makes the page itself scroll sideways. A table inside an
// overflow-x-auto wrapper is not that: its content is wider on purpose and the
// wrapper takes the scroll, so only boxes computing to overflow-x: visible are
// counted. Elements with no box of their own are skipped, because clientWidth is
// 0 on an inline and every one of them would read as an overflow.
//
// Tooltip bodies are taken out of the measurement first. They are absolutely
// positioned overlays that are meant to extend past the card, which is the whole
// reason the card no longer clips its own content, and they are laid out even
// while transparent, so leaving them in would report the panel's own layout as
// broken for doing the thing it was fixed to do. Their placement is a hover
// question, checked in a browser rather than here.
function overflowing(root: HTMLElement): string[] {
	const overlays = Array.from(root.querySelectorAll<HTMLElement>('[data-tooltip-body]'));
	for (const overlay of overlays) overlay.style.display = 'none';

	try {
		return Array.from(root.querySelectorAll<HTMLElement>('*'))
			.filter(element => {
				if (!element.clientWidth) return false;
				if (getComputedStyle(element).overflowX !== 'visible') return false;

				// A pixel of tolerance, because subpixel layout rounds against a
				// fractional container width and one pixel is not a broken page.
				return element.scrollWidth - element.clientWidth > 1;
			})
			.map(element => `${element.tagName.toLowerCase()}.${element.className}`);
	} finally {
		for (const overlay of overlays) overlay.style.display = '';
	}
}
