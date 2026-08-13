import { Component, OnDestroy, OnInit } from '@angular/core';
import { AdminService } from '../admin.service';
import { GeneralService } from 'src/app/services/general/general.service';
import { LicensesService } from 'src/app/services/licenses/licenses.service';

type PartitionOperation = 'partition' | 'unpartition';

interface PartitionRun {
	uid: string;
	table_name: string;
	operation: PartitionOperation;
	status: 'running' | 'completed' | 'failed';
	phase: string | null;
	notice_count: number;
	error: string | null;
	triggered_by: string;
	started_at: string;
	updated_at: string;
	completed_at: string | null;
}

@Component({
	selector: 'app-table-partitions',
	templateUrl: './table-partitions.component.html',
	standalone: false
})
export class TablePartitionsComponent implements OnInit, OnDestroy {
	tables = [
		{ uid: 'events', name: 'events' },
		{ uid: 'events_search', name: 'events_search' },
		{ uid: 'event_deliveries', name: 'event_deliveries' },
		{ uid: 'delivery_attempts', name: 'delivery_attempts' }
	];

	operations = [
		{ uid: 'partition', name: 'Partition' },
		{ uid: 'unpartition', name: 'Unpartition' }
	];

	selectedTable = 'events';
	selectedOperation: PartitionOperation = 'partition';

	// Typing the table name is the confirmation. Conversion rewrites the table and
	// cannot be interrupted safely, so it should not be one misplaced click away.
	confirmation = '';

	runs: PartitionRun[] = [];
	isStarting = false;
	isLoadingRuns = false;
	hasRetentionLicense = false;

	private pollInterval: any;

	constructor(private adminService: AdminService, private generalService: GeneralService, private licenseService: LicensesService) {}

	async ngOnInit() {
		// Refresh before reading, or a licensed instance shows the unlicensed gate
		// until the next full reload, because the cache is filled asynchronously by
		// the shell.
		await this.licenseService.loadAllLicenses();

		// Instance license, not the org intersection: the server gates this on the
		// deployment licenser alone, same as the CLI, and partitioning is an
		// instance-wide operation like queue monitoring.
		this.hasRetentionLicense = this.licenseService.hasInstanceLicense('RetentionPolicy');
		if (this.hasRetentionLicense) await this.loadRuns();
	}

	ngOnDestroy() {
		this.stopPolling();
	}

	get activeRun(): PartitionRun | undefined {
		return this.runs.find(run => run.status === 'running');
	}

	get canStart(): boolean {
		return !this.isStarting && !this.activeRun && this.confirmation.trim() === this.selectedTable;
	}

	async loadRuns() {
		// One request at a time. Polling and the refresh button share this path, and
		// a slow response landing after a newer one would show older progress than
		// the operator already saw.
		if (this.isLoadingRuns) return;

		this.isLoadingRuns = true;
		try {
			const response = await this.adminService.listPartitionRuns();
			this.runs = response.data ?? [];
			if (this.activeRun) this.startPolling();
			else this.stopPolling();
		} catch {
			// Leaving the list as it was is better than blanking a run an operator
			// is watching because one poll failed.
		} finally {
			this.isLoadingRuns = false;
		}
	}

	async startRun() {
		if (!this.canStart) return;

		this.isStarting = true;
		try {
			await this.adminService.startPartitionRun({ table: this.selectedTable, operation: this.selectedOperation });
			this.confirmation = '';
			this.generalService.showNotification({ message: `${this.selectedOperation} of ${this.selectedTable} started`, style: 'success' });
			await this.loadRuns();
		} catch (error: any) {
			// HttpService rejects with the API message as a plain string, and that
			// message is the actionable part: a 409 names the run already in flight.
			const message = typeof error === 'string' && error.length > 0 ? error : 'Failed to start partition run';
			this.generalService.showNotification({ message, style: 'error' });
		} finally {
			this.isStarting = false;
		}
	}

	private startPolling() {
		if (this.pollInterval) return;
		this.pollInterval = setInterval(() => this.loadRuns(), 5000);
	}

	private stopPolling() {
		if (!this.pollInterval) return;
		clearInterval(this.pollInterval);
		this.pollInterval = null;
	}

	elapsed(run: PartitionRun): string {
		const end = run.completed_at ? new Date(run.completed_at) : new Date();
		const seconds = Math.max(0, Math.floor((end.getTime() - new Date(run.started_at).getTime()) / 1000));

		const hours = Math.floor(seconds / 3600);
		const minutes = Math.floor((seconds % 3600) / 60);
		if (hours > 0) return `${hours}h ${minutes}m`;
		if (minutes > 0) return `${minutes}m ${seconds % 60}s`;
		return `${seconds}s`;
	}

	statusColor(status: PartitionRun['status']): 'success' | 'error' | 'warning' {
		if (status === 'completed') return 'success';
		if (status === 'failed') return 'error';
		return 'warning';
	}
}
