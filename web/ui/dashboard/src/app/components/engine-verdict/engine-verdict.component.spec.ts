import { ComponentFixture, TestBed } from '@angular/core/testing';

import { EngineFlow, EngineVerdict, EngineVerdictComponent, EngineWindow } from './engine-verdict.component';

// One measured interval. 4954ms because an engine reports the real elapsed time
// between its two readings rather than the period it was configured with.
function measured(overrides: Partial<EngineWindow> = {}): EngineFlow {
	return { measured: { windowMs: 4954, in: 0, out: 0, failed: 0, ...overrides } };
}

function verdict(overrides: Partial<EngineVerdict> = {}): EngineVerdict {
	return {
		accepting: true,
		reporting: true,
		sampledLabel: '2s ago',
		flow: measured(),
		outstanding: { known: true, value: 0 },
		...overrides
	};
}

describe('EngineVerdictComponent', () => {
	let fixture: ComponentFixture<EngineVerdictComponent>;
	let component: EngineVerdictComponent;

	beforeEach(async () => {
		await TestBed.configureTestingModule({ imports: [EngineVerdictComponent] }).compileComponents();

		fixture = TestBed.createComponent(EngineVerdictComponent);
		component = fixture.componentInstance;
	});

	afterEach(() => {
		fixture.destroy();
	});

	// The component's own detector, not fixture.detectChanges: these tests move
	// the input between passes on purpose, which is exactly what the fixture's
	// verification pass exists to reject.
	function render(input: EngineVerdict): string {
		component.verdict = input;
		fixture.changeDetectorRef.detectChanges();

		return (fixture.nativeElement as HTMLElement).textContent?.trim() ?? '';
	}

	// The absences this component exists to keep apart from a zero, and from each
	// other. An engine with no earlier sample has not said it is idle, an engine
	// that restarted has not said it is idle, and an engine that measured a zero
	// has. Each absence names its own cause, because every one of them is
	// knowable and a sentence that just reported an absence read as a fault in
	// the panel.
	describe('unknown is never zero', () => {
		it('names a first sample as the reason there is no interval', () => {
			const text = render(verdict({ flow: { absence: 'starting' }, outstanding: { known: true, value: 84088 } }));

			expect(component.state).toBe('starting');
			expect(text).toBe('Measuring. 84,088 outstanding.');
			expect(text).not.toContain('Idle');
			expect(text).not.toContain('not reported');
		});

		it('names a restart as the reason, distinctly from a first sample', () => {
			const text = render(verdict({ flow: { absence: 'restarted' }, outstanding: { known: true, value: 84088 } }));

			expect(component.state).toBe('restarted');
			expect(text).toBe('Measuring again. 84,088 outstanding.');
			expect(text).not.toContain('No earlier sample');
		});

		// The one absence with no benign explanation, so it is the only one that
		// still says the answer is unknown.
		it('says a partly reported interval is unknown', () => {
			const text = render(verdict({ flow: { absence: 'incomplete' }, outstanding: { known: false, value: 0 } }));

			expect(component.state).toBe('incomplete');
			expect(text).toBe('Throughput incomplete. Outstanding could not be read.');
			expect(text).not.toContain('Idle');
			expect(text).not.toContain('nothing outstanding');
		});

		// The one case that may claim idle, and only because both halves of the
		// claim were actually read: a measured interval that moved nothing, and a
		// backlog count that came back zero.
		//
		// The sentence says the interval was measured, because an engine that
		// looked and found nothing is a different fact from an engine that has
		// not looked, and on a deployment whose events never reach this engine
		// this is the only place that difference is stated.
		it('reports a measured zero as an engine that measured and found nothing', () => {
			const text = render(verdict({ outstanding: { known: true, value: 0 } }));

			expect(component.state).toBe('idle');
			expect(text).toBe('Idle. Nothing in the last ~5s, nothing outstanding.');
			// A measured zero is not a fault and must not be dressed as one.
			expect(component.tone).toBe('ok');
		});

		// The panel cannot see the instance's ingest rate, so it cannot say
		// traffic is bypassing this engine. It states its own reading and leaves
		// the comparison to the reader.
		it('does not assert that traffic is going elsewhere', () => {
			const text = render(verdict({ outstanding: { known: true, value: 0 } }));

			expect(text).not.toContain('If work is arriving');
			expect(text).not.toContain('bypass');
		});

		// A measured zero next to a backlog nobody could read is not idle. The
		// half that would justify "nothing outstanding" is missing.
		it('does not call a measured zero idle when the backlog could not be read', () => {
			const text = render(verdict({ outstanding: { known: false, value: 0 } }));

			expect(component.state).toBe('holding');
			expect(text).toContain('Nothing taken in and nothing completed in the last ~5s, outstanding could not be read.');
			expect(text).not.toContain('Idle');
		});
	});

	// in and out count different things: on the data plane, events arriving and
	// deliveries completing, with one event fanning out to a delivery per
	// matching subscription. A sentence pairing them as "1,240 in and 1,190 out"
	// invites a subtraction whose answer means nothing, so both are named for
	// what they count and neither is netted against the other.
	it('names what each side counts rather than pairing them as one quantity', () => {
		const text = render(
			verdict({
				flow: measured({ in: 1240, out: 1190 }),
				outstanding: { known: true, value: 50 }
			})
		);

		expect(component.state).toBe('draining');
		expect(text).toBe('Draining. 1,240 taken in and 1,190 completed in the last ~5s, 50 outstanding.');
	});

	// Fanout makes out legitimately larger than in, which a netting sentence
	// would have to report as an impossible surplus.
	it('reports more completed than taken in without calling it a deficit', () => {
		const text = render(
			verdict({
				flow: measured({ in: 3, out: 12 }),
				outstanding: { known: true, value: 4 }
			})
		);

		expect(component.state).toBe('draining');
		expect(text).toBe('Draining. 3 taken in and 12 completed in the last ~5s, 4 outstanding.');
		expect(text).not.toContain('more');
	});

	it('names failures inside a draining interval', () => {
		const text = render(
			verdict({
				flow: measured({ in: 288, out: 287, failed: 30 }),
				outstanding: { known: true, value: 583 }
			})
		);

		// The three window counts stay together and the level stays apart, so the
		// failures do not read as an explanation of the backlog beside them.
		expect(text).toBe('Draining. 288 taken in, 287 completed and 30 failed in the last ~5s, 583 outstanding.');
	});

	// An engine may count more than one way of not completing in that number. The
	// data plane sums deliveries it discarded without a send with deliveries that
	// failed after one, because the window answers whether accepted work left the
	// outstanding set and both of those do, so the sentence takes the word from
	// the caller rather than naming the count after the narrower outcome.
	it('lets the engine name what its failed count covers', () => {
		const text = render(
			verdict({
				flow: measured({ in: 288, out: 287, failed: 30 }),
				outstanding: { known: true, value: 583 },
				failedWord: 'failed or discarded'
			})
		);

		expect(text).toBe('Draining. 288 taken in, 287 completed and 30 failed or discarded in the last ~5s, 583 outstanding.');
	});

	// Completed nothing over the window against a backlog that is really there.
	// This is the state that caused a false alarm: every one of those 1,200 can
	// be a delivery whose next attempt is scheduled for later, which is a working
	// engine, and nothing in this contract separates that from a stall. So the
	// sentence reports what was measured and says what it cannot tell, rather
	// than calling a healthy engine stuck.
	it('reports an engine that completed nothing as an observation, not as a stall', () => {
		const text = render(
			verdict({
				flow: measured({ in: 20, out: 0 }),
				outstanding: { known: true, value: 1200 }
			})
		);

		expect(component.state).toBe('holding');
		expect(text).toBe('20 taken in and nothing completed in the last ~5s, 1,200 outstanding.');
		expect(text).not.toContain('Not draining');
		expect(text).not.toContain('stuck');
	});

	// An amber dot is an assertion of its own. Painting one over a backlog of
	// scheduled retries is the false alarm rendered in colour, so the state the
	// sentence declines to judge is coloured as unjudged.
	it('does not colour an unjudged backlog as a fault', () => {
		render(verdict({ flow: measured({ in: 20, out: 0 }), outstanding: { known: true, value: 1200 } }));

		expect(component.tone).toBe('unknown');
	});

	it('reports an engine that is up and not accepting', () => {
		const text = render(verdict({ accepting: false, flow: measured({ in: 999, out: 999 }) }));

		expect(component.state).toBe('stopped');
		// Distinct in words from the engine that is running and measured a zero,
		// which is the pair an operator most needs told apart.
		expect(text).toBe('Not accepting work.');
		expect(text).not.toContain('It measured the last');
		// The numbers beside a stopped engine describe a stop, so none of them
		// may reach the sentence.
		expect(text).not.toContain('999');
	});

	it('reports a stale sample as not reporting rather than as a rate', () => {
		const text = render(
			verdict({
				reporting: false,
				sampledLabel: '4m ago',
				flow: measured({ in: 1240, out: 1190 }),
				outstanding: { known: true, value: 50 }
			})
		);

		expect(component.state).toBe('silent');
		expect(text).toBe('Not reporting. Last sample 4m ago.');
		expect(text).not.toContain('1,240');
	});

	// The engine names its own window, so a plane sampling every five seconds must
	// not have its interval read aloud as a minute's worth of work. Marked
	// approximate, because the engine reports elapsed time and 4954ms is not 5s.
	it('names the window the engine actually sampled, as an approximation', () => {
		expect(render(verdict({ flow: measured({ windowMs: 4954, in: 4, out: 4 }), outstanding: { known: true, value: 1 } }))).toContain('in the last ~5s');
		expect(render(verdict({ flow: measured({ windowMs: 120000, in: 4, out: 4 }), outstanding: { known: true, value: 1 } }))).toContain('in the last ~2m');
		expect(render(verdict({ flow: measured({ windowMs: 0, in: 4, out: 4 }), outstanding: { known: true, value: 1 } }))).toContain('in the last sample');
	});

	// One interpolation of one getter, so the DOM cannot hold two verdicts. This
	// asserts the property rather than the implementation: whatever the state, the
	// rendered text is exactly the one sentence.
	it('paints exactly one sentence whatever the state', () => {
		const cases: EngineVerdict[] = [
			verdict({ accepting: false }),
			verdict({ reporting: false }),
			verdict({ flow: { absence: 'starting' } }),
			verdict({ flow: { absence: 'restarted' } }),
			verdict({ flow: { absence: 'incomplete' } }),
			verdict(),
			verdict({ flow: measured({ in: 1, out: 0 }), outstanding: { known: true, value: 9 } }),
			verdict({ flow: measured({ in: 5, out: 5 }), outstanding: { known: true, value: 2 } })
		];

		const states = new Set<string>();
		for (const input of cases) {
			const text = render(input);
			states.add(component.state);
			expect(text).toBe(component.sentence);
			expect(text.length).toBeGreaterThan(0);
		}

		expect(states.size).toBe(cases.length);
	});

	// A stop is a problem. An absence with a benign cause is not: the engine may
	// be draining perfectly and simply have no window to prove it over yet, so
	// the dot must not read as a fault while it waits.
	it('colours a stop as a warning and an unmeasured interval as unknown, never as a fault', () => {
		render(verdict({ accepting: false }));
		expect(component.tone).toBe('warning');

		render(verdict({ flow: { absence: 'starting' } }));
		expect(component.tone).toBe('unknown');

		render(verdict({ flow: { absence: 'restarted' } }));
		expect(component.tone).toBe('unknown');

		render(verdict({ flow: { absence: 'incomplete' } }));
		expect(component.tone).toBe('unknown');

		render(verdict({ flow: measured({ in: 5, out: 5 }), outstanding: { known: true, value: 2 } }));
		expect(component.tone).toBe('ok');
	});
});
