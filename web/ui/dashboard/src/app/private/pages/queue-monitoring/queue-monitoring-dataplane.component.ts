import { CommonModule } from '@angular/common';
import { Component, EventEmitter, Input, OnDestroy, OnInit, Output } from '@angular/core';

import { TagComponent } from 'src/app/components/tag/tag.component';
import { TooltipComponent } from 'src/app/components/tooltip/tooltip.component';
import { EngineCount, EngineFlow, EngineFlowAbsence, EngineVerdict, EngineVerdictComponent, engineCount, intervalWindowLabel } from 'src/app/components/engine-verdict/engine-verdict.component';
import { AdminService } from 'src/app/private/pages/admin/admin.service';

// The shapes the server publishes. Nothing here names a stage, a writer or a
// backlog: the data plane chooses those names and this page renders whatever it
// was given, so a plane can change its own vocabulary without touching the UI.
interface DataPlaneStage {
	name: string;
	queued: number;
	waiting: number;
	workers: number;
	partitions: number;
	partition_capacity: number;
	deepest_partition: number;
}

interface DataPlaneWriter {
	name: string;
	pending: number;
	failures: number;
}

interface DataPlaneMetric {
	name: string;
	value: number;
}

interface DataPlaneBacklog {
	name: string;
	count: number;
	// False when the count could not be read. Rendering it as 0 would report an
	// empty backlog, which is the failure this whole page exists to surface.
	known: boolean;
	as_of: string;
}

// The server omits a section it has nothing for. The lists are required here
// because absence is normalised once, on the way in: a template that wrote
// `x ?? []` per binding would hand the renderer a new array on every pass.
interface DataPlaneReplica {
	replica: string;
	mode: string;
	running: boolean;
	sampled_at: string;
	age_seconds: number;
	stale: boolean;
	stages: DataPlaneStage[];
	writers: DataPlaneWriter[];
	counters: DataPlaneMetric[];
	gauges: DataPlaneMetric[];
	outstanding: DataPlaneBacklog[];
}

// Two tooltip lines and the accessible label carrying both. The tooltip body
// only appears on hover and focus, so the label is the only route a screen
// reader or a tablet has to the same text.
interface Explainer {
	reason: string;
	detail: string;
	aria: string;
}

// One of the three numbers under the verdict.
//
// Two explanations, on two separate hover targets, because they answer two
// different questions and stacking them on one target would have them fight:
// about sits on the label and says what the metric means, always, whatever the
// value; absent sits on the value and says why that particular number is not
// there.
//
// value null is the absence, and the template renders the dash for it rather
// than a word: "Not reported" printed under every label makes the quiet state
// the loudest thing on the panel, and the more a plane omits the more the page
// shouts about it.
interface FlowNumber {
	label: string;
	value: string | null;
	about: Explainer;
	// null exactly when value is not, so the dash and its reason cannot be
	// rendered apart.
	absent: Explainer | null;
	// Facts about the number, each on its own line under it, in the order given.
	// The interval numbers name the window they were counted over; Outstanding
	// names how old its oldest waiting retry is and when the next one is due,
	// which is what turns a level into something an operator can judge.
	notes: string[];
}

// What the plane said about its retry backlog's schedule. Three answers, and the
// difference between the last two is the whole point: a backlog with nothing in
// it has no oldest retry, and that is a reading rather than a gap.
type RetrySchedule = { kind: 'reported'; oldestAgeMs: number; nextDueInMs: number } | { kind: 'empty' } | { kind: 'unknown' };

// A replica with everything derived from it computed once. The derived values
// are objects and arrays, so a template getter would hand the renderer a new
// identity on every change detection pass; building them on the way in gives
// each one a single identity for as long as the reply it came from is on screen.
interface DataPlaneRow {
	replica: DataPlaneReplica;
	verdict: EngineVerdict;
	numbers: FlowNumber[];

	// Two lists, never merged. holding is where items are sitting; failures are
	// totals the plane counts across everything it serves. Putting a failure
	// count under a heading that says work is stuck asserts a causal link this
	// contract cannot support, and that assertion is what caused a false alarm.
	holding: string[];
	failures: string[];

	// What throughputOf made of this replica's gauges, and the gauges it did not
	// consume. Diagnostics renders both from here rather than reaching for the
	// gauge list itself, so the table cannot show numbers the verdict above it
	// has just called absent.
	flow: EngineFlow;
	gauges: DataPlaneMetric[];

	// Why there is no interval, decided once for the whole row. The dashes in the
	// numbers and the line in diagnostics both read it, so the card cannot give
	// two accounts of one absence.
	absentFlow: Explained | null;
}

// What this panel tells the page around it. The page needs it to decide whether
// the Data plane segment exists at all and which segment an operator lands on,
// and neither question can be answered from inside a segment that may not be
// rendered.
export interface DataPlaneReport {
	reports: boolean;
	// False when the counts below come from a read that failed. reports can still
	// be true on the same report: a plane that answered once is still there when
	// a poll blips, so the segment stays, but the counts describe nothing and a
	// consumer must not state them as facts about now. Zero replicas after a
	// failed read is unknown, not an empty plane.
	known: boolean;
	replicas: number;
	accepting: number;
}

// loading: no read has landed yet. Nothing renders, because most instances turn
// out to have no plane and a spinner would flash a panel onto every one of them.
// hidden: the server wires no data plane monitoring at all (501), so there is
// nothing to poll for and nothing to show.
// empty: the read worked and no replica has published. A deployment running no
// plane that publishes cannot be told apart from one whose replicas all went
// away, so the panel is not rendered, but polling continues.
// unknown: the read failed, so the last numbers no longer describe the present.
type DataPlaneState = 'loading' | 'ready' | 'empty' | 'hidden' | 'unknown';

// The plane names its own counters, so no dictionary can be trusted to list
// them. These are the words that mean something went wrong whatever the plane
// calls the thing it happened to. A counter carrying none of them stays in
// diagnostics, which is the safe direction to be wrong in: a missed promotion
// costs a click, a wrong one puts a healthy number under "where it is stuck".
const PROBLEM_WORDS = ['error', 'errors', 'fail', 'failed', 'failure', 'failures', 'abandoned', 'dropped', 'lost', 'rejected', 'timeout', 'timeouts'];

// The glyph for a value there is nothing to print for, matching the no-data
// cells the dashboard already has. A dash conventionally means no data, which a
// zero does not, and it holds the row rhythm so a reader can still scan the
// labels beside it.
const NO_VALUE = '-';

// Why a value is a dash. Every absence on this panel comes from here, so the
// same glyph never has to stand for two reasons without saying which: a plane
// that is not accepting, one on its first sample, one that restarted underneath
// the sampler, one that sent part of an interval, and a count the plane could
// not read. The first line is the reason and the second is what it means for the
// number, matching the dashboard's existing no-data cells.
const ABSENT_REASONS: Record<EngineFlowAbsence, Explained> & { stopped: Explained; unread: Explained; unreadCount: Explained } = {
	// A replica that is not accepting publishes no interval, and the run-scoped
	// sections are dropped on the way in, so the gauge list is empty for a reason
	// that has nothing to do with sampling. Saying a number appears on the next
	// sample would promise one from a plane that has stopped taking work, which is
	// the opposite of what the verdict beside it says.
	stopped: {
		reason: 'Plane not accepting.',
		detail: 'This replica reported that it is not accepting work, so it measured no interval. No number appears here until it is accepting again.'
	},
	// The first two say a number is coming and that nothing is wrong. A dash an
	// operator cannot account for reads as a broken panel, and the wire says
	// exactly which of the two it is, so neither has to be vague about it.
	starting: {
		reason: 'No earlier sample yet.',
		detail: 'This is the first sample since the plane started, so there is nothing to compare it against. A number appears on the next sample. Nothing is wrong.'
	},
	restarted: {
		reason: 'Plane restarted.',
		detail: 'A total went backwards, so the interval spanning the restart was discarded rather than reported as a rate. A number appears on the next sample. Nothing is wrong.'
	},
	incomplete: {
		reason: 'Reported incompletely.',
		detail: 'The plane sent part of an interval, and a count without the window it covers is not a rate. This is unknown rather than zero.'
	},
	unread: {
		reason: 'Not read.',
		detail: 'At least one backlog count could not be read, so the total is unknown rather than zero.'
	},
	unreadCount: {
		reason: 'Not read.',
		detail: 'The plane could not read this count, so it is unknown rather than zero.'
	}
};

// What each metric means, available on its label whether or not the value is
// known. A number an operator cannot interpret is not information: "Outstanding
// is growing" is only alarming to a reader who knows it counts work not yet
// done, so where a direction has a meaning it is stated.
//
// Only the metrics whose meaning this build actually knows are described. The
// plane names its own stages, counters and backlogs, so no copy can be written
// for those without inventing it, and an invented explanation is worse than
// none.
// The two interval numbers count different things, so each says what it counts
// and the pair says outright that it must not be subtracted. One accepted event
// fans out to a delivery per matching subscription, so deliveries out routinely
// exceeds events in, and a reader who takes these for one quantity measured twice
// reads that surplus as impossible or a deficit as a backlog forming.
const METRIC_MEANINGS = {
	in: {
		reason: 'Events accepted in the last measured interval.',
		detail: 'Counted once the plane has taken responsibility for the event, and counted as events. Not comparable with Deliveries out: one event fans out to a delivery per matching subscription, so the two are the ends of the plane rather than two sides of a sum. A count for the interval, not a rate.'
	},
	out: {
		reason: 'Deliveries completed in the last measured interval.',
		detail: 'Successful deliveries only, counted as deliveries. It can exceed Events in, because one event fans out to a delivery per matching subscription, so the difference between the two means nothing. Zero over a window is not by itself a stall: deliveries waiting on a retry schedule complete when they are due, not when they are counted. A count for the interval, not a rate.'
	},
	// Deliberately not "growing means the plane is behind". A delivery waiting on
	// a retry schedule is outstanding for as long as its schedule says, so under
	// steady load the count climbs to the arrival rate times that schedule and
	// then sits there, which is the system working. Reading that plateau as a
	// stuck plane is the false alarm this copy exists to prevent, so the copy
	// says what the number counts and points at the schedule lines, which are
	// where a reader can tell a backlog draining on time from one that is not.
	outstanding: {
		reason: 'Work accepted and not yet done.',
		detail: 'The durable backlog, so it survives a restart. It counts deliveries waiting on a retry schedule as well as work waiting to be attempted, and a scheduled retry is future work with a due time rather than work held up. A steady arrival rate against a retry schedule holds a roughly constant number here, so a count that rises and then settles is what a working plane looks like. The lines under it say how old the oldest waiting retry is and when the next one is due, which is what separates a backlog draining on schedule from one that is not.'
	}
};

// Said where someone staring at a row of zeros will read it, on the identity
// those zeros belong to. Which process accepted the work is the whole
// difference between a plane that is broken and a plane nothing is being sent
// to, and nothing else in the product would tell an operator that an agent
// running a plane only takes what was posted to its own port.
//
// Worded for the contract rather than for one plane: any plane can publish this
// shape, so the note says what is true of a replica and then names the
// agent-port case, instead of asserting an agent port on a plane that may not
// have one.
// What the queue carries instead is on the segment caveat above, not repeated
// here.
const REPLICA_SCOPE: Explained = {
	reason: 'This replica only counts work its own process accepted.',
	detail: 'On an agent running a data plane that is work posted to the agent HTTP port. Work that reaches the instance any other way is carried by the queue instead, so this replica can be accepting nothing while the instance itself is busy. The numbers below are a fact about this replica, not about the instance.'
};

// The gauges a plane publishes its measured interval under, and the only place
// those names are spelled. A plane reports throughput through the same
// vocabulary-neutral gauge list as everything else rather than through a section
// of its own, and these names carry the concept rather than any plane's
// components, so a queue-based plane can publish the same set.
//
// The first four are one compound value and are omitted together. The fifth is
// published alone, with those four absent, so a restart sample is also an
// unmeasured sample: the two coincide rather than being alternatives.
const THROUGHPUT_GAUGES = {
	windowMs: 'throughput_window_ms',
	accepted: 'throughput_accepted',
	delivered: 'throughput_delivered',
	failed: 'throughput_failed',
	restarted: 'throughput_restarted'
} as const;

// The four that make up the interval. Any one of them missing means the interval
// is unknown, so this list is what the accessor requires rather than defaulting a
// member to zero: a count without the window it covers is not a rate, and a
// window without its counts is not a measurement.
const WINDOW_GAUGES: readonly string[] = [THROUGHPUT_GAUGES.windowMs, THROUGHPUT_GAUGES.accepted, THROUGHPUT_GAUGES.delivered, THROUGHPUT_GAUGES.failed];

// The two gauges that describe the retry backlog's schedule, and the backlog
// entry they describe. Same compound rule as the interval: published together or
// omitted together, because an oldest age without a due time says nothing about
// whether the backlog is draining and a due time without an age says nothing
// about how long it has not been.
const RETRY_GAUGES = {
	oldestAgeMs: 'outstanding_oldest_retry_age_ms',
	nextDueInMs: 'outstanding_next_retry_due_in_ms'
} as const;

// The one backlog name this panel spells, and it spells it because the two
// gauges above describe that backlog and no other. Which of the plane's backlogs
// has a schedule is a fact about the schedule, not vocabulary the renderer
// invented, and a plane that names its retry backlog something else simply
// publishes no schedule this panel can attribute.
const RETRY_BACKLOG = 'deliveries_retry';

// How overdue the plane's own reading has to be before the panel says so as well
// as showing it. One sample cannot see persistence, so the threshold has to be
// large enough that a single reading carries it: a retry can be a little overdue
// simply because the scanner has not reached it yet inside its lease window, and
// the plane publishes every few seconds, so a minute is an order of magnitude
// above the lateness that is ordinary and cannot be produced by it.
const OVERDUE_NOTE_MS = 60_000;

// The two lifetime failure totals, which the plane reports separately because
// they are different events: one delivery was never sent, the other was sent and
// did not succeed. They are named here rather than left to the problem-word rule
// below, which would promote one of them and leave the other in diagnostics.
const FAILURE_COUNTERS = {
	discarded: 'deliveries_discarded',
	failed: 'deliveries_failed'
} as const;

// Words that read worse in sentence case than shouted. This is a spelling list,
// not a vocabulary list: it does not need to know what any plane calls anything.
const ACRONYMS = new Set(['lsn', 'id', 'ids', 'db', 'sql', 'http', 'url', 'tls', 'cdc', 'api', 'ttl']);

// An omitted section becomes an empty list once, here, so each list keeps one
// identity for as long as the reply it came from is on screen.
//
// The run-scoped sections are dropped when a replica is not accepting work, and
// dropped here rather than trusted to arrive empty. Stage depth, writer backlog,
// the live gauges and the last interval's flow describe a plane that is running;
// on one that stopped they are the last thing it saw, and rendering them beside
// a "not accepting" tag invites reading them as current. Counters and the
// durable backlog stay: a total still describes the run that ended, and rows
// outstanding in the database are outstanding whether or not anything is
// draining them.
function sections(replica: any): DataPlaneReplica {
	const running = replica.running === true;
	return {
		...replica,
		stages: running ? replica.stages ?? [] : [],
		writers: running ? replica.writers ?? [] : [],
		gauges: running ? replica.gauges ?? [] : [],
		counters: replica.counters ?? [],
		outstanding: replica.outstanding ?? []
	};
}

// The one reader of the throughput gauges on this panel. The verdict sentence,
// the In and Out cells and the diagnostics table all take their answer from
// here, so the three cannot hold separate opinions of what the plane published:
// before this existed, one path looked for a `throughput` section the plane does
// not send while another iterated the gauges it does, and the card reported "not
// reported" above the numbers themselves.
//
// It answers one question three ways: a measured interval, a measured interval
// that happens to be zero, which is a real answer and prints as 0, or no
// measurement, which carries the reason there is none and prints as a dash. Every
// reason is knowable from this one sample, so nothing downstream has to report an
// unexplained absence.
function throughputOf(replica: DataPlaneReplica): EngineFlow {
	const read = (name: string) => readGauge(replica, name);

	const windowMs = read(THROUGHPUT_GAUGES.windowMs);
	const accepted = read(THROUGHPUT_GAUGES.accepted);
	const delivered = read(THROUGHPUT_GAUGES.delivered);
	const failed = read(THROUGHPUT_GAUGES.failed);

	// The restart gauge is published alone, so it is read before the window is
	// judged missing: it is the reason the window is missing, and it is a more
	// useful thing to say than that the plane has only just started.
	if (read(THROUGHPUT_GAUGES.restarted)) return { absence: 'restarted' };

	// None of the four is the plane's first sample, which is the state it is in
	// for one interval after every start. Some of the four is a partial publish,
	// which nothing on the wire explains, so the two are kept apart.
	const present = WINDOW_GAUGES.filter(name => replica.gauges.some(gauge => gauge.name === name)).length;
	if (!present) return { absence: 'starting' };

	// An interval of no length carries no rate, so it joins the partial publish
	// rather than becoming a window a label would have to name.
	if (windowMs === null || windowMs <= 0 || accepted === null || delivered === null || failed === null) return { absence: 'incomplete' };

	return { measured: { windowMs, in: accepted, out: delivered, failed } };
}

// Number.isFinite rather than a presence check, because a gauge that arrived as
// a string or a NaN is not a measurement either.
function readGauge(replica: DataPlaneReplica, name: string): number | null {
	const gauge = replica.gauges.find(candidate => candidate.name === name);
	return gauge && Number.isFinite(gauge.value) ? Number(gauge.value) : null;
}

function readCounter(replica: DataPlaneReplica, name: string): number {
	const counter = replica.counters.find(candidate => candidate.name === name);
	return counter && Number.isFinite(counter.value) ? Number(counter.value) : 0;
}

// The one reader of the retry schedule, for the same reason throughputOf is the
// one reader of the interval.
//
// reported: the retry backlog was read, it has rows, and the plane published
// both gauges over them. The count and both readings come from one statement
// over one set of rows, so they agree by construction.
// empty: the backlog was read and has no rows. There is no oldest retry and
// nothing is due, which is an answer rather than a gap, so it must not render as
// the absence dash. The publisher does not send zeros for this, deliberately: a
// zero age already means a retry created this instant, which is a different fact
// from there being none, and reading an absence as a zero would put those two
// back together.
// unknown: the backlog could not be read, the plane is not running so the gauges
// carry no meaning and were cleared, or rows exist without the gauges that
// describe them, which the contract says cannot happen and so is a bug rather
// than a state to render. All three render nothing rather than a number, and the
// first is already carried by the dash on the count itself.
function retryScheduleOf(replica: DataPlaneReplica): RetrySchedule {
	const retries = replica.outstanding.find(backlog => backlog.name === RETRY_BACKLOG);
	if (!retries || !retries.known) return { kind: 'unknown' };

	// Checked before the gauges, so a plane that published a schedule alongside
	// an empty backlog still reads as empty. Rows are what an age describes, and
	// there are none.
	if (retries.count <= 0) return { kind: 'empty' };

	const oldestAgeMs = readGauge(replica, RETRY_GAUGES.oldestAgeMs);
	const nextDueInMs = readGauge(replica, RETRY_GAUGES.nextDueInMs);
	if (oldestAgeMs === null || nextDueInMs === null) return { kind: 'unknown' };

	return { kind: 'reported', oldestAgeMs, nextDueInMs };
}

// The gauges left for the diagnostics table once the accessor has taken the ones
// it reads. Rendering them there as well would put the plane's raw gauge names in
// front of an operator beside the same numbers already named above, which is the
// arrangement that let the table contradict the verdict in the first place.
function diagnosticGauges(replica: DataPlaneReplica): DataPlaneMetric[] {
	const consumed: readonly string[] = [...Object.values(THROUGHPUT_GAUGES), ...Object.values(RETRY_GAUGES)];
	return replica.gauges.filter(gauge => !consumed.includes(gauge.name));
}

@Component({
	selector: 'convoy-queue-monitoring-dataplane',
	imports: [CommonModule, TagComponent, TooltipComponent, EngineVerdictComponent],
	templateUrl: './queue-monitoring-dataplane.component.html'
})
export class QueueMonitoringDataplaneComponent implements OnInit, OnDestroy {
	// Whether this panel is the segment on screen. It polls either way: the page
	// reads this panel's report to decide whether the segmented control exists
	// and which segment is the default, so the poll cannot be tied to being
	// visible. Defaults true so the panel still renders on its own.
	@Input() active = true;

	@Output() reported = new EventEmitter<DataPlaneReport>();

	state: DataPlaneState = 'loading';
	rows: DataPlaneRow[] = [];
	staleAfterSeconds = 0;

	// Collapsed by default, per replica. Diagnostics is where today's whole panel
	// went, and it is opened by a reader who has decided the verdict is not
	// enough.
	private opened = new Set<string>();

	private timer?: ReturnType<typeof setInterval>;
	// Every response is checked against the request that is current when it
	// lands, so a slow first load cannot overwrite a newer refresh.
	private fetchId = 0;
	// ngOnInit awaits the first read before scheduling, so a view destroyed
	// while that read is in flight would otherwise start a timer after
	// ngOnDestroy has already run, and nothing would ever clear it.
	private destroyed = false;
	// Whether a read has ever found a replica in this session. A failed read is
	// not evidence that a plane exists, so on a queue-only instance the first
	// failure must not grow a Data plane segment; a failure after the plane has
	// answered keeps it, because the plane is still there and the read is what
	// broke.
	private everReported = false;

	constructor(private readonly adminService: AdminService) {}

	async ngOnInit(): Promise<void> {
		await this.load();
		this.schedule();
	}

	ngOnDestroy(): void {
		this.destroyed = true;
		this.stop();
	}

	private stop(): void {
		if (this.timer) clearInterval(this.timer);
		this.timer = undefined;
	}

	get polling(): boolean {
		return !!this.timer;
	}

	// A backlog that was not read is unknown, and it renders as the same dash the
	// numbers above it use, for the same reason: the word repeated down a column
	// of counts reads louder than the counts.
	readonly unreadCount = ABSENT_REASONS.unreadCount;

	readonly replicaScope = explainer('Replica scope', REPLICA_SCOPE);

	backlogLabel(backlog: DataPlaneBacklog): string {
		return backlog.known ? engineCount(backlog.count) : NO_VALUE;
	}

	backlogAria(backlog: DataPlaneBacklog): string {
		return `${this.label(backlog.name)}: ${this.unreadCount.reason} ${this.unreadCount.detail}`;
	}

	// The plane reports 0 workers when a stage runs one goroutine per item. That
	// is a mode, not an absence, so it must not read as "no workers".
	workersLabel(stage: DataPlaneStage): string {
		return stage.workers ? `${stage.workers}` : 'per item';
	}

	ageLabel(replica: DataPlaneReplica): string {
		const seconds = Math.max(0, Math.round(replica.age_seconds));
		if (seconds < 60) return `${seconds}s ago`;

		return `${Math.round(seconds / 60)}m ago`;
	}

	// Underscores to spaces, sentence case, and never SCREAMING_SNAKE. A name
	// this build has never seen still renders as a phrase, which is what keeps
	// the contract vocabulary-neutral: the UI stops shouting names it does not
	// know without having to know them.
	label(name: string): string {
		const words = (name ?? '')
			.replace(/[_\-.]+/g, ' ')
			.trim()
			.toLowerCase()
			.split(/\s+/)
			.filter(word => !!word)
			.map(word => (ACRONYMS.has(word) ? word.toUpperCase() : word));

		if (!words.length) return 'Unnamed';

		const first = words[0];
		const head = ACRONYMS.has(first.toLowerCase()) ? first : first.charAt(0).toUpperCase() + first.slice(1);

		return [head, ...words.slice(1)].join(' ');
	}

	isOpen(replica: string): boolean {
		return this.opened.has(replica);
	}

	toggleDiagnostics(replica: string): void {
		if (this.opened.has(replica)) this.opened.delete(replica);
		else this.opened.add(replica);
	}

	async load(): Promise<void> {
		const id = ++this.fetchId;

		try {
			const response = await this.adminService.getDataPlaneStatus();
			if (id !== this.fetchId) return;

			const replicas = (response.data?.replicas ?? []).map(sections);
			this.rows = replicas.map((replica: DataPlaneReplica) => this.row(replica));
			this.staleAfterSeconds = response.data?.stale_after_seconds ?? 0;
			this.state = this.rows.length ? 'ready' : 'empty';
			if (this.state === 'ready') this.everReported = true;
			else this.everReported = false;
		} catch (error: any) {
			if (id !== this.fetchId) return;

			// 501 is the server saying this deployment wires no data plane
			// monitoring, which is a definitive answer and not a failure.
			if (error?.response?.status === 501) {
				this.rows = [];
				this.everReported = false;
				this.state = 'hidden';
				this.stop();
				this.publishReport();
				return;
			}

			// Anything else is unknown, and the rows already on screen are
			// dropped with it: a failed refresh must not leave stale depth
			// answering for the present. everReported is left alone, because a
			// failed read says nothing about whether a plane exists.
			this.rows = [];
			this.state = 'unknown';
		}

		this.pruneOpened();
		this.publishReport();
	}

	// Ids that are no longer on screen are dropped, so a replica that comes and
	// goes cannot grow this set for the life of the tab.
	private pruneOpened(): void {
		const visible = new Set(this.rows.map(row => row.replica.replica));
		this.opened.forEach(id => {
			if (!visible.has(id)) this.opened.delete(id);
		});
	}

	private publishReport(): void {
		const reports = this.state === 'ready' || (this.state === 'unknown' && this.everReported);

		this.reported.emit({
			reports,
			// Every state but 'unknown' is a read that answered, including the empty
			// one and the 501: zero replicas from a read that worked is a known
			// zero. Only a failed read leaves the counts describing nothing.
			known: this.state !== 'unknown',
			replicas: this.rows.length,
			accepting: this.rows.filter(row => row.replica.running).length
		});
	}

	// The accessor is called once per replica per reply and its answer is handed
	// to every consumer below, so the verdict, the numbers, the stuck line and
	// diagnostics are reading one value rather than each re-deriving it.
	private row(replica: DataPlaneReplica): DataPlaneRow {
		const outstanding = this.outstandingTotal(replica);
		const flow = throughputOf(replica);

		return {
			replica,
			// The window sums two terminal outcomes on this plane, so the sentence
			// is told what to call them rather than naming the count after the
			// narrower one.
			verdict: { accepting: replica.running, reporting: !replica.stale, sampledLabel: this.ageLabel(replica), flow, outstanding, failedWord: 'failed or discarded' },
			numbers: this.flowNumbers(replica, flow, outstanding),
			holding: this.holdingLines(replica),
			failures: this.failureLines(replica, flow),
			flow,
			gauges: diagnosticGauges(replica),
			absentFlow: flow.absence ? absenceLines(replica, flow.absence) : null
		};
	}

	// The reason diagnostics prints where the interval would be, read off the row
	// so it cannot differ from the dash's tooltip upstairs.
	absentReason(row: DataPlaneRow): string {
		return row.absentFlow ? `${row.absentFlow.reason} ${row.absentFlow.detail}` : '';
	}

	// The durable backlogs summed, because the verdict asks one question and the
	// operator compares one number against a queue depth.
	//
	// An absent section is absent: a plane that reported no backlog has not
	// reported an empty one, so an empty list is unknown rather than zero. One
	// unread count makes the sum a lower bound, which is also unknown, because a
	// lower bound presented as a total is exactly how a full backlog comes to
	// read as an empty one.
	private outstandingTotal(replica: DataPlaneReplica): EngineCount {
		if (!replica.outstanding.length) return { known: false, value: 0 };

		let total = 0;
		for (const backlog of replica.outstanding) {
			if (!backlog.known) return { known: false, value: 0 };
			total += backlog.count;
		}

		return { known: true, value: total };
	}

	// Events in, Deliveries out, Outstanding. Three states per number and never
	// two of them at once: no measured interval, which is the dash and carries
	// the reason there is none; a measured value; and a measured zero, which
	// prints as 0 rather than as a dash, because an idle plane is a real and
	// meaningful answer and a dash would make it look broken.
	//
	// The labels name their units because the first two count different things,
	// and no note on either compares it against the other: any phrasing of the
	// two together would be a subtraction, and the plane's own fanout makes that
	// meaningless. Outstanding's notes describe its own retry rows, nothing else
	// on the row.
	private flowNumbers(replica: DataPlaneReplica, flow: EngineFlow, outstanding: EngineCount): FlowNumber[] {
		// The two interval numbers are absent together or measured together,
		// because they come from one compound value the plane publishes or omits
		// as a whole. Branching once here is what makes that structural rather
		// than a rule each cell has to remember.
		const interval: FlowNumber[] = flow.absence
			? [absentNumber('Events in', METRIC_MEANINGS.in, absenceLines(replica, flow.absence)), absentNumber('Deliveries out', METRIC_MEANINGS.out, absenceLines(replica, flow.absence))]
			: [
					// A measured zero on the intake gets a second line saying it was
					// measured. The dash and the 0 are already different glyphs, but a
					// reader scanning a card of zeros cannot see which of them the plane
					// looked for and which it never reported, and on this number that is
					// the difference between a quiet instance and one whose events are
					// all reaching Convoy somewhere else.
					measuredNumber('Events in', METRIC_MEANINGS.in, flow.measured.in, flow.measured.windowMs, 'measured, nothing reached this replica'),
					measuredNumber('Deliveries out', METRIC_MEANINGS.out, flow.measured.out, flow.measured.windowMs)
				];

		// The snapshot's own age, from the server that read sampled_at, carries the
		// schedule forward to now without the browser's clock entering into it.
		const snapshotAgeMs = Number.isFinite(replica.age_seconds) ? Math.max(0, replica.age_seconds * 1000) : 0;

		return [...interval, outstandingNumber(outstanding, retryScheduleOf(replica), snapshotAgeMs)];
	}

	// Where work is sitting right now, phrased as places rather than table rows,
	// and rendered only when something is actually non-zero.
	//
	// This is the section that was headed "Where it is stuck" and listed a
	// failure count under it. Both halves of that were claims the data does not
	// support: an item queued in a stage is an item queued in a stage, and a
	// failure counter is a lifetime total that is not attributed to any of the
	// items sitting here. Reading those together turned a plane working exactly
	// as designed into a twenty minute incident, so the two are separate
	// sections now and neither asserts a cause.
	private holdingLines(replica: DataPlaneReplica): string[] {
		const lines: string[] = [];

		for (const stage of replica.stages) {
			const parts: string[] = [];
			if (stage.queued) parts.push(`${engineCount(stage.queued)} queued`);
			if (stage.waiting) parts.push(`${engineCount(stage.waiting)} senders waiting`);
			if (parts.length) lines.push(`${this.label(stage.name)}: ${parts.join(', ')}`);
		}

		for (const writer of replica.writers) {
			if (writer.pending) lines.push(`${this.label(writer.name)} writer: ${engineCount(writer.pending)} pending`);
		}

		return lines;
	}

	// Failures, as counts and nothing more. Every line here is a total the plane
	// reports for itself, across every endpoint and project it serves, so none of
	// them can be attributed to the backlog above: a paused endpoint's deliveries
	// are discarded before dispatch and never join the outstanding set at all,
	// yet they can dominate this number. The section says so rather than leaving
	// the adjacency to imply otherwise.
	//
	// The two lifetime totals are stated as the different events they are. Half
	// of why this card read as an incident was one number holding both: it was
	// dominated by a paused endpoint's discards, which never join the outstanding
	// set at all, while the backlog beside it belonged to an endpoint that was
	// retrying normally.
	private failureLines(replica: DataPlaneReplica, flow: EngineFlow): string[] {
		const lines: string[] = [];

		const failedBatches = replica.writers.reduce((total, writer) => total + (writer.failures ?? 0), 0);
		if (failedBatches) lines.push(`${engineCount(failedBatches)} write batches failed`);

		const discarded = readCounter(replica, FAILURE_COUNTERS.discarded);
		if (discarded) lines.push(`${engineCount(discarded)} deliveries discarded before any attempt`);

		const failed = readCounter(replica, FAILURE_COUNTERS.failed);
		if (failed) lines.push(`${engineCount(failed)} deliveries failed after an attempt`);

		// Only a measured interval can say anything ended in it, and the accessor
		// is what decides that: an unmeasured interval carries no window here
		// rather than a window of zeroes, so there is no unmeasured zero to guard
		// against.
		//
		// Named for both outcomes. The window counts a discard the same as an
		// exhausted retry on purpose, because it answers whether accepted work
		// left the outstanding set, and calling it a failure count now that the
		// lifetime totals are split would make one of the two invisible here.
		const window = flow.measured;
		if (window?.failed) {
			lines.push(`${engineCount(window.failed)} deliveries failed or discarded in the last ${intervalWindowLabel(window.windowMs)}`);
		}

		// Failures and recovery errors are the only counters that always mean
		// something went wrong, so they are promoted here whenever they are
		// non-zero. Everything else stays in diagnostics. The two named above are
		// skipped: the word rule would promote the failed one a second time and
		// leave the discarded one out, which is the conflation this split exists
		// to end.
		const named: readonly string[] = Object.values(FAILURE_COUNTERS);
		for (const counter of replica.counters) {
			if (named.includes(counter.name)) continue;
			if (counter.value && isProblemCounter(counter.name)) lines.push(`${this.label(counter.name)}: ${engineCount(counter.value)}`);
		}

		return lines;
	}

	// One interval owner: the server derives staleness from the same sample time
	// the publishers use, so the poll is read back from it rather than configured
	// again here. Floored so a tiny sample time cannot turn this page into load.
	private schedule(): void {
		if (this.destroyed || this.state === 'hidden') return;

		const seconds = Math.max(5, Math.round(this.staleAfterSeconds / 3));
		this.timer = setInterval(() => void this.load(), seconds * 1000);
	}
}

// The two authored lines, before the metric they belong to is known.
type Explained = { readonly reason: string; readonly detail: string };

// The accessible label names the metric, because a screen reader reaching a
// tooltip button hears only what is in here: the two lines alone would not say
// which of the three numbers they describe.
function explainer(label: string, lines: Explained): Explainer {
	return { reason: lines.reason, detail: lines.detail, aria: `${label}: ${lines.reason} ${lines.detail}` };
}

// Which absence this is. The accessor names the sampling reason, and it cannot
// see the one case that is not about sampling: a replica that is not accepting
// had its run-scoped gauges dropped on the way in, so it reads as the same empty
// list a plane on its first sample publishes. Telling that reader a number
// arrives on the next sample promises one from a plane that has stopped taking
// work, and contradicts the verdict beside it. Both the dashes and diagnostics
// come through here, so the row cannot give two accounts of one absence.
function absenceLines(replica: DataPlaneReplica, absence: EngineFlowAbsence): Explained {
	return replica.running ? ABSENT_REASONS[absence] : ABSENT_REASONS.stopped;
}

// A dash and the reason for it. The reason is decided above rather than by the
// cell, so a cell cannot invent an explanation the wire did not give.
function absentNumber(label: string, about: Explained, lines: Explained): FlowNumber {
	return { label, value: null, about: explainer(label, about), absent: explainer(label, lines), notes: [] };
}

// The window is named on the number rather than assumed, and marked approximate
// by intervalWindowLabel, because the plane reports the elapsed time it actually
// measured rather than the period it was configured with.
// zeroNote is for a number whose zero is worth reading as a reading. It is only
// added when the plane measured the interval, so it can never appear beside a
// dash and can never be mistaken for an explanation of an absent value.
function measuredNumber(label: string, about: Explained, value: number, windowMs: number, zeroNote?: string): FlowNumber {
	const notes = [`last ${intervalWindowLabel(windowMs)}`];
	if (value === 0 && zeroNote) notes.push(zeroNote);

	return { label, value: engineCount(value), about: explainer(label, about), absent: null, notes };
}

// The backlog as a level, with the schedule of its retries under it. The level
// alone is still not described as stuck, behind or growing, because it counts
// deliveries whose next attempt is scheduled for the future; what separates those
// from work genuinely held up is the schedule, and the schedule says it in its
// own lines rather than by colouring the count.
//
// Two minutes old with something due in thirty seconds is a backlog draining on
// time. An hour old with nothing due for a long time is the shape of one nothing
// is draining. That is the distinction "Where it is stuck" claimed to make and
// could not, and it lives here, beside the number it is about.
function outstandingNumber(outstanding: EngineCount, schedule: RetrySchedule, snapshotAgeMs: number): FlowNumber {
	const about = explainer('Outstanding', METRIC_MEANINGS.outstanding);
	// The schedule lines survive a total that could not be read, unlike the
	// measured note on the intake, which is suppressed beside a dash. They are
	// not the same kind of line: that one would be read as an account of the
	// missing value, while these describe retry rows the plane did read. A total
	// is unknown as soon as any one backlog is, so refusing to state a schedule
	// that was read would throw away the most useful thing on the card over an
	// unread count somewhere else.
	const notes = scheduleNotes(schedule, snapshotAgeMs);
	if (!outstanding.known) return { label: 'Outstanding', value: null, about, absent: explainer('Outstanding', ABSENT_REASONS.unread), notes };

	return { label: 'Outstanding', value: engineCount(outstanding.value), about, absent: null, notes };
}

// The schedule as two plain readings, and a third line only when one sample can
// carry it.
//
// Both readings are carried forward from the sample rather than recomputed from
// the browser's clock. The plane published an age and a time-to-due instead of
// two timestamps precisely so no consumer has to line up two clocks, and neither
// number crossed a clock boundary to be produced; subtracting the browser's now
// from anything here would fold plane-to-browser skew into a number an operator
// pages on. The snapshot's own age is the only thing added, and it comes from
// the same server that read sampled_at.
//
// The third line says the next retry is overdue by more than a scan cycle could
// explain. It stops short of calling the backlog stuck, because this sample
// cannot see whether that item is being worked right now, and it stops short of
// calling it fine, because nothing being due for that long is the shape of a
// backlog nothing is draining.
//
// That line is judged on the plane's own reading rather than on the reading
// carried forward, deliberately. Carrying forward is a display convenience and
// says nothing about what happened since: an old sample of a healthy backlog
// crosses any threshold you like just by sitting there, and a panel that turned
// its own staleness into an overdue claim would be the false alarm again with a
// new cause.
function scheduleNotes(schedule: RetrySchedule, snapshotAgeMs: number): string[] {
	if (schedule.kind === 'unknown') return [];
	if (schedule.kind === 'empty') return ['no retries waiting'];

	const oldest = schedule.oldestAgeMs + snapshotAgeMs;
	const due = schedule.nextDueInMs - snapshotAgeMs;

	const notes = [`oldest retry ${durationLabel(oldest)} old`, due >= 0 ? `next due in ${durationLabel(due)}` : `next due ${durationLabel(-due)} ago`];

	if (-schedule.nextDueInMs > OVERDUE_NOTE_MS) notes.push('overdue by longer than a scan cycle explains');

	return notes;
}

// Whole units, floored, because these are read beside each other and a value
// that rounds 59 minutes up to an hour makes the pair inconsistent. Flooring
// also keeps an age from ever reading older than it is.
function durationLabel(milliseconds: number): string {
	const seconds = Math.max(0, Math.floor(milliseconds / 1000));
	if (seconds < 60) return `${seconds}s`;

	const minutes = Math.floor(seconds / 60);
	if (minutes < 60) return `${minutes}m`;

	const hours = Math.floor(minutes / 60);
	const rest = minutes % 60;

	return rest ? `${hours}h ${rest}m` : `${hours}h`;
}

function isProblemCounter(name: string): boolean {
	return (name ?? '')
		.toLowerCase()
		.split(/[^a-z]+/)
		.some(word => PROBLEM_WORDS.includes(word));
}
