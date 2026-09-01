import { Component, Input } from '@angular/core';

// One measured interval of an engine's flow, in the vocabulary of the question
// an operator is asking: is accepted work draining, and if not, where is it
// stuck. Every number is that interval alone, never a total and never a rate.
//
// in and out are counted at the two ends of the engine and are not the same
// quantity measured twice: on the data plane one accepted event fans out to a
// delivery per matching subscription, so out legitimately exceeds in. Nothing
// here may be subtracted from anything else here, which is why this carries no
// net and the sentence never states one.
//
// Both halves of Queue Monitoring can fill this. The data plane fills it from
// the interval its publisher sampled; the queue fills it from pending, processed
// and failed. Nothing here names a stage, a lane, a partition or a task, so
// neither half can pull the sentence into its own vocabulary.
export interface EngineWindow {
	// How long the interval actually was, in milliseconds, as the engine measured
	// it. The sentence names the window from this rather than assuming a minute.
	windowMs: number;

	in: number;
	out: number;

	// Work that left the engine without completing. It is named by the caller
	// through failedWord, because an engine may count more than one way of not
	// completing here: the data plane sums deliveries it discarded without a send
	// with deliveries that failed after one, since the window answers whether
	// accepted work left the outstanding set and both of those do.
	failed: number;
}

// Why an engine has no measured interval. Every cause is benign and every one of
// them is knowable, so the sentence names which rather than reporting an
// unexplained absence: an operator told the panel could not measure yet waits a
// few seconds, while one told nothing was reported goes looking for a fault.
//
// starting: no earlier reading to difference, which is the first sample after
// the engine started.
// restarted: a monotonic total came back lower, so the engine underneath
// restarted and the interval spanning it was discarded.
// incomplete: the engine sent part of an interval. A count without its window is
// not a rate, so this is the one absence the panel cannot explain, and it is kept
// separate rather than dressed up as one of the two benign causes above.
export type EngineFlowAbsence = 'starting' | 'restarted' | 'incomplete';

// An engine's flow: either the interval it measured, or the reason there is
// none. One value rather than a window beside a flag, so there is no pair to
// keep in agreement and no arrangement that says both.
export type EngineFlow = { measured: EngineWindow; absence?: never } | { measured?: never; absence: EngineFlowAbsence };

// A count and whether it was read. A backlog rendered as 0 because the query
// timed out reads as an empty queue, which is the failure this whole screen
// exists to surface.
export interface EngineCount {
	known: boolean;
	value: number;
}

export interface EngineVerdict {
	// False is the engine saying it is up and not taking work, which makes every
	// number beside it describe a stop rather than a rate.
	accepting: boolean;

	// False when the engine's last sample is too old to answer for the present.
	reporting: boolean;

	// How long ago that sample was taken, already phrased by the caller ('4m
	// ago'), because staleness is the one value read on the client and the caller
	// is what knows the units it wants.
	sampledLabel: string;

	// The measured interval or the reason there is none. An absence is not a zero:
	// an engine that could not measure has not told anyone it is idle.
	flow: EngineFlow;

	outstanding: EngineCount;

	// What the window's failed count is called in the sentence. Defaults to the
	// plain word for an engine whose count is only failures; an engine that sums
	// more than one terminal outcome into it must say so here rather than let the
	// sentence name it after the narrower one.
	failedWord?: string;
}

// stopped: the engine says it is not accepting.
// silent: its last sample is too old to describe the present.
// starting, restarted, incomplete: no measured interval, named by cause.
// idle, holding, draining: it published a measured interval.
//
// holding is not "stuck". An engine that completed nothing over one window with
// work outstanding may be stalled, or every outstanding item may be a retry with
// a due time in the future, which is future work on a schedule. This contract
// carries nothing that separates those, so the state is named for what was
// observed and the sentence points at the outstanding number rather than
// interpreting it. The data plane does publish the age of its oldest waiting
// retry and when the next one is due, and says so beside the count those belong
// to; this sentence stays out of it because the queue engine has no equivalent,
// and a verdict that firms up for one engine and not the other is how the two
// halves of one tab drift into two languages.
//
// There is no growing state. A backlog rising would have to come from
// subtracting out from in, which counts two different things, or from
// differencing two of the client's own polls, which reports a restart as the
// backlog falling. Outstanding is reported as the level it is instead.
export type EngineVerdictState = 'stopped' | 'silent' | EngineFlowAbsence | 'idle' | 'holding' | 'draining';

@Component({
	selector: 'convoy-engine-verdict',
	imports: [],
	templateUrl: './engine-verdict.component.html'
})
export class EngineVerdictComponent {
	@Input({ required: true }) verdict!: EngineVerdict;

	// One sentence, resolved by one chain of early returns. The template has a
	// single interpolation of it, so there is no arrangement of inputs that can
	// paint two states at once or none: whichever branch answers first is the
	// only text on the line.
	get state(): EngineVerdictState {
		const verdict = this.verdict;

		// Ordered as the spec's table is. A publisher saying it is not accepting
		// has made a definitive statement about itself; staleness is a fact about
		// the sample, and it is reported beside this line rather than instead of it.
		if (!verdict.accepting) return 'stopped';
		if (!verdict.reporting) return 'silent';

		const flow = verdict.flow;
		if (flow.absence) return flow.absence;

		const window = flow.measured;

		// Idle is the only state allowed to claim nothing is outstanding, so it
		// needs the backlog to have actually been read. A failure inside the window
		// is movement, so it is not idle either.
		if (!window.in && !window.out && !window.failed && verdict.outstanding.known && !verdict.outstanding.value) return 'idle';

		// Nothing came out over the window while work is outstanding or the
		// backlog could not be read. Reported as the observation it is: the
		// sentence does not call this stuck, because a backlog of retries with
		// future due times looks exactly like this and is healthy.
		if (!window.out) return 'holding';

		return 'draining';
	}

	// The two units are named in every sentence that states a count. In is work
	// arriving and out is work completing, and on the data plane those are events
	// and deliveries, so a sentence that said "288 in and 287 out" would invite a
	// subtraction that fanout makes meaningless.
	get sentence(): string {
		const verdict = this.verdict;
		const window = verdict.flow.measured;

		switch (this.state) {
			// Three different things an operator has to tell apart, and only one
			// of them is a fault. This engine is up and refusing work; the next is
			// up and its samples have stopped; idle below is up, measuring, and
			// finding nothing to do. Rendering any two of them the same way is how
			// a misconfiguration hides.
			case 'stopped':
				return 'Not running. This engine is not accepting work, so nothing new is entering it.';
			case 'silent':
				return `Not reporting. Last sample ${verdict.sampledLabel}.`;
			// Both absences say what happened and what it means for the reader,
			// and neither reads as a fault: an interval is coming, and until it
			// arrives the engine may well be draining perfectly.
			case 'starting':
				return `Measuring. No earlier sample from this engine to compare against, so there is no interval yet. Next sample carries one. ${sentenceStart(this.outstandingClause())}.`;
			case 'restarted':
				return `Measuring again. The engine restarted, so the interval spanning the restart was discarded. Next sample carries one. ${sentenceStart(this.outstandingClause())}.`;
			// The one absence with no benign explanation, kept apart from the two
			// that have one rather than being softened into them.
			case 'incomplete':
				return `Throughput reported incompletely, so whether work is draining is unknown. ${sentenceStart(this.outstandingClause())}.`;
			// A measured zero is a reading, not a gap, and the difference is the
			// whole point: an engine that sampled an interval and found nothing is
			// telling the operator that work is reaching Convoy some other way, or
			// not at all. The last clause is conditional because this panel cannot
			// see the instance's ingest rate and so cannot make the comparison
			// itself.
			case 'idle':
				return `Running and idle. It measured the last ${this.window()} and accepted nothing, nothing outstanding. If work is arriving on this instance, it is not coming through this engine.`;
			// The observation, then the reason it is not a conclusion. Reading
			// this as a stall cost an operator twenty minutes on a plane that was
			// working exactly as designed, so the sentence says outright what it
			// cannot tell rather than leaving the reader to assume the worst.
			case 'holding':
				return `${window?.in ? `${engineCount(window.in)} accepted` : 'Nothing accepted'} and nothing completed in the last ${this.window()}${window?.failed ? `, ${engineCount(window.failed)} ${this.failedWord()}` : ''}, ${this.outstandingClause()}. Work waiting on a schedule is outstanding too, so read what that number is made of before taking this as held up.`;
			default:
				return `Draining. ${engineCount(window?.in ?? 0)} accepted${window?.failed ? `, ` : ' and '}${engineCount(window?.out ?? 0)} completed${window?.failed ? ` and ${engineCount(window.failed)} ${this.failedWord()}` : ''} in the last ${this.window()}, ${this.outstandingClause()}.`;
		}
	}

	// A state that describes a stop is a problem, and everything the sentence
	// cannot resolve is unknown. Colour never carries information the sentence
	// does not already say, and it never asserts what the sentence declines to:
	// an engine that has not measured an interval yet may be draining perfectly,
	// and an engine holding a backlog of scheduled retries is doing its job. An
	// amber dot on either is the false alarm, painted.
	get tone(): 'ok' | 'warning' | 'unknown' {
		switch (this.state) {
			case 'stopped':
			case 'silent':
				return 'warning';
			case 'starting':
			case 'restarted':
			case 'incomplete':
			case 'holding':
				return 'unknown';
			default:
				return 'ok';
		}
	}

	get dotClass(): string {
		if (this.tone === 'warning') return 'bg-warning-9';
		if (this.tone === 'unknown') return 'bg-new.text-secondary';

		return 'bg-success-9';
	}

	private outstandingClause(): string {
		const outstanding = this.verdict.outstanding;

		return outstanding.known ? `${engineCount(outstanding.value)} outstanding` : 'outstanding could not be read';
	}

	private window(): string {
		return intervalWindowLabel(this.verdict.flow.measured?.windowMs ?? 0);
	}

	private failedWord(): string {
		return this.verdict.failedWord ?? 'failed';
	}
}

// The engine names its own window, because a plane sampling every five seconds
// and one sampling every minute publish the same field. A publisher that sent no
// window length gets the neutral phrasing rather than a made-up minute.
//
// Approximate on purpose, and marked as approximate. A sampler reports the real
// elapsed time between its two readings, so a five second period arrives
// anywhere from 4948 to 5008 ms: reading '4.948s' aloud teaches an operator
// nothing, and reading '5s' claims an interval the engine never promised. The
// tilde is what keeps the rounding from becoming a claim.
export function intervalWindowLabel(milliseconds: number): string {
	const seconds = Math.round(milliseconds / 1000);
	if (milliseconds <= 0 || seconds <= 0) return 'sample';
	if (seconds < 60) return `~${seconds}s`;

	return `~${Math.round(seconds / 60)}m`;
}

// A fixed locale, because these are operator numbers read beside each other and
// the grouping must not change with the browser's language. Exported so the
// panels feeding this component group their own numbers identically: the verdict
// sentence and the numbers under it are read as one paragraph.
export function engineCount(value: number): string {
	return value.toLocaleString('en-US');
}

function sentenceStart(clause: string): string {
	return clause.charAt(0).toUpperCase() + clause.slice(1);
}
