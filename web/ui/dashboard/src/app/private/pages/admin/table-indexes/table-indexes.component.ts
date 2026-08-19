import { Component, OnDestroy, OnInit } from '@angular/core';
import { AdminService } from '../admin.service';
import { GeneralService } from 'src/app/services/general/general.service';
import { MaintenanceRun } from '../runs/run.model';

interface InvalidIndex {
	table: string;
	name: string;
	busy: boolean;
}

interface DroppedIndex {
	table: string;
	name: string;
	dropped_at: string;
	unique: boolean;
}

@Component({
	selector: 'app-table-indexes',
	templateUrl: './table-indexes.component.html',
	standalone: false
})
export class TableIndexesComponent implements OnInit, OnDestroy {
	invalid: InvalidIndex[] = [];
	dropped: DroppedIndex[] = [];
	runs: MaintenanceRun[] = [];

	// Three states per list, not two: answered, failed, and neither yet. An empty
	// list read as an all-clear would tell an operator their indexes are fine
	// because a request failed, and one read as a failure would report an error
	// for a request that has not been made. Failure is only ever set by a catch.
	reportKnown = false;
	reportFailed = false;
	runsKnown = false;
	runsFailed = false;

	isLoading = false;
	isLoadingRuns = false;

	// The index a rebuild has been asked for, so only that row shows its button as
	// busy. A rebuild takes hours, and the request returns as soon as the run is
	// recorded, so this clears when the response lands, not when the work ends.
	starting: string | null = null;

	private pollInterval: any;
	private destroyed = false;

	constructor(private adminService: AdminService, private generalService: GeneralService) {}

	async ngOnInit() {
		await this.load();
		await this.loadRuns();
	}

	ngOnDestroy() {
		this.destroyed = true;
		this.stopPolling();
	}

	// No license gate. An interrupted index build leaves the same invalid index on
	// any instance, and the server gates this on instance admin alone.
	async load() {
		if (this.isLoading) return;

		this.isLoading = true;
		try {
			const response = await this.adminService.listIndexes();
			this.invalid = response.data?.invalid ?? [];
			this.dropped = response.data?.dropped ?? [];
			this.reportKnown = true;
			this.reportFailed = false;
		} catch {
			// Leave the last known report on screen rather than blanking it, and
			// keep reportKnown false so nothing reads the blank as an all-clear.
			this.reportKnown = false;
			this.reportFailed = true;
		} finally {
			this.isLoading = false;
		}
	}

	get activeRun(): MaintenanceRun | undefined {
		return this.runs.find(run => run.status === 'running');
	}

	// One rebuild at a time for the whole instance, and a conversion holds the same
	// slot, so the buttons go down for either. They also stay down while the run
	// list is unknown: a read that failed cannot report the slot as free, and the
	// conversion that took it may have been started from the other page or a shell.
	get canStart(): boolean {
		return !this.starting && this.runsKnown && !this.activeRun;
	}

	async rebuild(index: DroppedIndex) {
		if (!this.canStart) return;

		this.starting = index.name;
		try {
			await this.adminService.startIndexRebuild({ index: index.name });
			this.generalService.showNotification({ message: `Started rebuilding ${index.name}`, style: 'success' });
			await this.loadRuns();
		} catch (error: any) {
			// HttpService rejects with the API message as a plain string, and that
			// message is the actionable part: a 409 names the run already in flight.
			const message = typeof error === 'string' && error.length > 0 ? error : 'Failed to start rebuild';
			this.generalService.showNotification({ message, style: 'error' });
		} finally {
			this.starting = null;
		}
	}

	async loadRuns() {
		// One request at a time. Polling and the refresh button share this path, and
		// a slow response landing after a newer one would show older progress than
		// the operator already saw.
		if (this.isLoadingRuns) return;

		const wasRunning = !!this.activeRun;

		this.isLoadingRuns = true;
		try {
			const response = await this.adminService.listPartitionRuns();
			this.runs = response.data ?? [];
			this.runsKnown = true;
			this.runsFailed = false;
			if (this.activeRun) {
				this.startPolling();
			} else {
				this.stopPolling();
				// A rebuild that just finished changed what is owed, and a
				// conversion that finished can change which indexes are invalid.
				if (wasRunning) await this.load();
			}
		} catch {
			// Leaving the list as it was is better than blanking a run an operator
			// is watching because one poll failed. It stops answering for the present
			// though, the same way a failed report does above: an earlier success
			// does not survive a later failure.
			this.runsKnown = false;
			this.runsFailed = true;
		} finally {
			this.isLoadingRuns = false;
		}
	}

	private startPolling() {
		// A refresh in flight when the operator leaves the tab resolves after
		// ngOnDestroy and would otherwise re-arm the interval on a component
		// nothing is rendering, polling the admin API until the page is reloaded.
		if (this.destroyed || this.pollInterval) return;
		this.pollInterval = setInterval(() => this.loadRuns(), 5000);
	}

	private stopPolling() {
		if (!this.pollInterval) return;
		clearInterval(this.pollInterval);
		this.pollInterval = null;
	}

	// Only the rebuilds are shown here; conversions belong to the partitions page,
	// and this list is what the buttons above act on.
	get rebuildRuns(): MaintenanceRun[] {
		return this.runs.filter(run => run.operation === 'rebuild_index');
	}

	get uniqueDroppedCount(): number {
		return this.dropped.filter(index => index.unique).length;
	}

	// A build in progress is marked invalid until it finishes, so it is reported
	// and left alone rather than presented as a failure.
	state(index: InvalidIndex): string {
		return index.busy ? 'being built now, leave it' : 'abandoned by a failed build';
	}

	// A missing index costs speed. A missing unique index also costs the
	// uniqueness it enforced, which is the part worth saying out loud.
	costs(index: DroppedIndex): string {
		return index.unique ? 'Slower queries, and its key is no longer unique' : 'Slower queries';
	}
}
