import { CommonModule } from '@angular/common';
import { Component, OnInit } from '@angular/core';
import { ActivatedRoute, Router } from '@angular/router';

import { LoaderModule } from 'src/app/private/components/loader/loader.module';
import { AdminService } from 'src/app/private/pages/admin/admin.service';
import { LicensesService } from 'src/app/services/licenses/licenses.service';
import { RbacService } from 'src/app/services/rbac/rbac.service';

import { QueueMonitoringAsynqmonComponent } from './queue-monitoring-asynqmon.component';
import { QueueMonitoringBrokerComponent } from './queue-monitoring-broker.component';
import { DataPlaneReport, QueueMonitoringDataplaneComponent } from './queue-monitoring-dataplane.component';

type QueueMonitoringSurface = 'loading' | 'postgres' | 'redis';

// Two engines, one at a time on screen. Never a mode switch that hides one: when
// a data plane runs both are live, the plane owning HTTP events for its projects
// while the queue still carries broadcast and dynamic events, broker sources,
// replays and batch retries, meta events, emails, retention and backups.
export type QueueMonitoringSegment = 'dataplane' | 'queue';

const SEGMENT_PARAM = 'engine';

@Component({
	selector: 'convoy-queue-monitoring',
	imports: [CommonModule, LoaderModule, QueueMonitoringAsynqmonComponent, QueueMonitoringBrokerComponent, QueueMonitoringDataplaneComponent],
	templateUrl: './queue-monitoring.component.html'
})
export class QueueMonitoringComponent implements OnInit {
	licensed = false;
	// Whether the license read has been attempted, not that it succeeded:
	// loadAllLicenses swallows transport errors, so a failed read falls back to
	// the cache and reads as unlicensed on a cold one. What it buys is that
	// "requires a license key" waits for the read, not the initial false.
	licenseKnown = false;
	// True while the reads that decide which panel shows are still in flight.
	// Not the inverse of licenseKnown: the non-admin return clears this without
	// setting licenseKnown, because "requires a license key" must not be stated
	// from a verdict that was never read. Every exit from ngOnInit clears it, so
	// no path leaves the loader on screen; neither read it awaits can reject.
	isLoadingLicense = true;
	surface: QueueMonitoringSurface = 'loading';

	// What the data plane panel last reported about itself. The panel polls even
	// while the Queue segment is on screen, because these two questions cannot be
	// answered from inside a segment that may not be rendered.
	planeReports = false;
	planeReplicas = 0;
	planeAccepting = 0;
	// Whether the two counts above describe a read that succeeded. A failed poll
	// keeps the segment, because the plane is still there, but it cannot be
	// allowed to say zero replicas are reporting.
	planeCountsKnown = false;

	// The segment the operator chose, or the default once the plane has answered.
	// Read through the segment getter, never directly, so no state can leave the
	// Data plane segment selected on an instance whose plane does not report.
	private chosen: QueueMonitoringSegment = 'queue';
	// True once the choice belongs to the operator, from the URL or a click. The
	// default rule may not overrule it.
	private pinned = false;
	// True once the default rule has run. It runs on the first report only: a
	// plane that stops accepting later must not move the segment out from under
	// someone reading it.
	private defaulted = false;

	constructor(
		private readonly adminService: AdminService,
		private readonly licenses: LicensesService,
		private readonly rbacService: RbacService,
		private readonly router: Router,
		private readonly route: ActivatedRoute
	) {}

	async ngOnInit(): Promise<void> {
		this.readSegmentFromUrl();

		const role = await this.rbacService.getUserRole();
		if (role !== 'INSTANCE_ADMIN') {
			this.isLoadingLicense = false;
			this.router.navigateByUrl('/');
			return;
		}

		await this.licenses.loadAllLicenses();
		this.licensed = this.licenses.hasInstanceLicense('AsynqMonitoring');
		this.licenseKnown = true;
		this.isLoadingLicense = false;
		if (!this.licensed) return;

		await this.resolveSurface();
	}

	// Which view is on screen. Derived rather than stored: the Data plane segment
	// only exists while a plane reports, so a shared link naming it, or a plane
	// that goes away while the tab is open, both land on the queue instead of on
	// a blank panel.
	get segment(): QueueMonitoringSegment {
		return this.planeReports ? this.chosen : 'queue';
	}

	get usePostgresUi(): boolean {
		return this.surface === 'postgres';
	}

	// One line about the engine that is not on screen, so the operator can see
	// that the answer might be on the other side.
	get otherEngineLine(): string {
		if (this.segment === 'dataplane') {
			return 'The queue still carries non-HTTP work.';
		}

		const tail = 'HTTP events on their projects do not use this queue.';
		if (!this.planeCountsKnown) return `Data plane last read failed. ${tail}`;

		const replicas = `${this.planeReplicas} data plane ${this.planeReplicas === 1 ? 'replica' : 'replicas'}`;

		return `${replicas}, ${this.planeAccepting} accepting. ${tail}`;
	}

	// The default segment is whichever engine owns HTTP events on this instance,
	// read from what the plane published rather than from a flag the dashboard
	// does not have. A plane with a replica accepting work is the engine those
	// events go through, so that is where the operator lands; a plane that
	// reports and is accepting nothing has handed nothing off, so the queue is
	// the engine carrying HTTP events and the queue is the landing view.
	//
	// Applied once. A later poll that changes the answer leaves the segment
	// alone, because moving the view under a reader is worse than landing them
	// one click from the other half.
	onPlaneReported(report: DataPlaneReport): void {
		this.planeReports = report.reports;
		this.planeCountsKnown = report.known;
		// Held from the last read that succeeded, so a blipped poll does not turn
		// the line beside the control into a claim that the plane emptied.
		if (report.known) {
			this.planeReplicas = report.replicas;
			this.planeAccepting = report.accepting;
		}

		if (!report.reports) {
			this.defaulted = false;
			return;
		}

		// Only a read that succeeded can say which engine owns HTTP events. A
		// failed one leaves the rule unapplied so the next good read decides,
		// rather than reading its zero as a plane accepting nothing.
		if (this.pinned || this.defaulted || !report.known) return;

		this.defaulted = true;
		this.chosen = report.accepting > 0 ? 'dataplane' : 'queue';
	}

	selectSegment(next: QueueMonitoringSegment): void {
		if (next === this.chosen) return;

		this.chosen = next;
		this.pinned = true;
		// An explicit value on every write, never a null clear, so the merge
		// cannot leave the previous segment behind in the URL. The snapshot is
		// read once in ngOnInit and never again, so this navigation cannot be
		// judged against a snapshot it has already replaced.
		this.router.navigate([], {
			relativeTo: this.route,
			queryParams: { [SEGMENT_PARAM]: next },
			queryParamsHandling: 'merge',
			replaceUrl: true
		});
	}

	// A shared link lands on the view it was shared from. An unrecognised value is
	// ignored rather than pinned, so a truncated or hand-edited URL falls back to
	// the default rule instead of freezing the page on a segment nobody chose.
	private readSegmentFromUrl(): void {
		const param = this.route.snapshot?.queryParams?.[SEGMENT_PARAM];
		if (param !== 'dataplane' && param !== 'queue') return;

		this.chosen = param;
		this.pinned = true;
	}

	// Redis keeps the existing asynqmon iframe. Postgres uses the native broker
	// page because asynqmon reads Redis directly and is not wired for queue_jobs.
	private async resolveSurface(): Promise<void> {
		this.surface = 'loading';

		try {
			const response = await this.adminService.getQueueStats();
			this.surface = response.data?.provider === 'postgres' ? 'postgres' : 'redis';
		} catch {
			// Fail open to the legacy iframe: Redis deployments always had it, and
			// a transient stats read must not hide asynqmon behind an empty gate.
			this.surface = 'redis';
		}
	}
}
