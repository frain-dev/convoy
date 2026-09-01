import { CommonModule } from '@angular/common';
import { Component, OnDestroy, OnInit } from '@angular/core';

import { TagComponent } from 'src/app/components/tag/tag.component';
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

// loading: no read has landed yet. Nothing renders, because most instances turn
// out to have no plane and a spinner would flash a panel onto every one of them.
// hidden: the server wires no data plane monitoring at all (501), so there is
// nothing to poll for and nothing to show.
// empty: the read worked and no replica has published. A deployment running no
// plane that publishes cannot be told apart from one whose replicas all went
// away, so the panel is not rendered, but polling continues.
// unknown: the read failed, so the last numbers no longer describe the present.
type DataPlaneState = 'loading' | 'ready' | 'empty' | 'hidden' | 'unknown';

// An omitted section becomes an empty list once, here, so each list keeps one
// identity for as long as the reply it came from is on screen.
//
// The run-scoped sections are dropped when a replica is not accepting work, and
// dropped here rather than trusted to arrive empty. Stage depth, writer backlog
// and the live gauges describe a plane that is running; on one that stopped they
// are the last thing it saw, and rendering them beside a "not accepting" tag
// invites reading them as current. Counters and the durable backlog stay: a
// total still describes the run that ended, and rows outstanding in the database
// are outstanding whether or not anything is draining them.
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

@Component({
	selector: 'convoy-queue-monitoring-dataplane',
	imports: [CommonModule, TagComponent],
	templateUrl: './queue-monitoring-dataplane.component.html'
})
export class QueueMonitoringDataplaneComponent implements OnInit, OnDestroy {
	state: DataPlaneState = 'loading';
	replicas: DataPlaneReplica[] = [];
	staleAfterSeconds = 0;

	private timer?: ReturnType<typeof setInterval>;
	// Every response is checked against the request that is current when it
	// lands, so a slow first load cannot overwrite a newer refresh.
	private fetchId = 0;
	// ngOnInit awaits the first read before scheduling, so a view destroyed
	// while that read is in flight would otherwise start a timer after
	// ngOnDestroy has already run, and nothing would ever clear it.
	private destroyed = false;

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

	// A backlog that was not read is unknown. The template asks for the label
	// rather than the number so there is one place this decision is made.
	backlogLabel(backlog: DataPlaneBacklog): string {
		return backlog.known ? `${backlog.count}` : 'unknown';
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

	async load(): Promise<void> {
		const id = ++this.fetchId;

		try {
			const response = await this.adminService.getDataPlaneStatus();
			if (id !== this.fetchId) return;

			this.replicas = (response.data?.replicas ?? []).map(sections);
			this.staleAfterSeconds = response.data?.stale_after_seconds ?? 0;
			this.state = this.replicas.length ? 'ready' : 'empty';
		} catch (error: any) {
			if (id !== this.fetchId) return;

			// 501 is the server saying this deployment wires no data plane
			// monitoring, which is a definitive answer and not a failure.
			if (error?.response?.status === 501) {
				this.replicas = [];
				this.state = 'hidden';
				this.stop();
				return;
			}

			// Anything else is unknown, and the rows already on screen are
			// dropped with it: a failed refresh must not leave stale depth
			// answering for the present.
			this.replicas = [];
			this.state = 'unknown';
		}
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
