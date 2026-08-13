import { Component, OnDestroy, OnInit } from '@angular/core';
import { FormBuilder, FormGroup } from '@angular/forms';
import { Subscription } from 'rxjs';
import { AdminService } from '../admin.service';
import { GeneralService } from 'src/app/services/general/general.service';
import { LicensesService } from 'src/app/services/licenses/licenses.service';

type PartitionOperation = 'partition' | 'unpartition';

const TABLE_NAMES = ['events', 'events_search', 'event_deliveries', 'delivery_attempts'];

const PARTITION = { uid: 'partition', name: 'Partition' };
const UNPARTITION = { uid: 'unpartition', name: 'Unpartition' };
const ALL_OPERATIONS = [PARTITION, UNPARTITION];

interface PartitionTable {
	name: string;
	partitioned: boolean;
	adopted: boolean;
}

interface PartitionStep {
	message: string;
	at: string;
}

interface PartitionRun {
	uid: string;
	table_name: string;
	operation: PartitionOperation;
	status: 'running' | 'completed' | 'failed';
	phase: string | null;
	steps: PartitionStep[] | null;
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
	tables = TABLE_NAMES.map(name => ({ uid: name, name }));

	// Narrowed to the one operation the selected table's shape allows, once the
	// states load. A partitioned table cannot be partitioned again and the server
	// refuses it, so offering it is an invitation to type a table name and read
	// an error.
	//
	// Assigned rather than derived in a getter: convoy-select re-initialises on
	// every new options array, so a getter would rebuild it on each change
	// detection pass.
	operations = ALL_OPERATIONS;

	// convoy-select injects ControlContainer without @Optional, so these controls
	// only work inside a form group. Typing the table name is the confirmation:
	// a conversion changes the shape of the largest tables in the schema and runs
	// for hours, so it should not be one misplaced click away.
	partitionForm: FormGroup;

	runs: PartitionRun[] = [];
	tableStates: PartitionTable[] = [];
	isStarting = false;
	isLoadingRuns = false;
	hasRetentionLicense = false;

	private pollInterval: any;
	private tableChanges: Subscription;
	private destroyed = false;
	private expandedRuns = new Set<string>();

	constructor(private adminService: AdminService, private generalService: GeneralService, private licenseService: LicensesService, private formBuilder: FormBuilder) {
		this.partitionForm = this.formBuilder.group({
			table: ['events'],
			operation: ['partition'],
			confirmation: ['']
		});

		// Each table is at a different point in its conversion, so the operation
		// that is possible changes with the table, not only when the states load.
		this.tableChanges = this.partitionForm.get('table')!.valueChanges.subscribe(() => this.applyOperations());
	}

	get selectedTable(): string {
		return this.partitionForm.value.table;
	}

	get selectedOperation(): PartitionOperation {
		return this.partitionForm.value.operation;
	}

	// Partitioning event_deliveries attaches the existing table as a partition
	// instead of copying it, so it neither loses writes nor needs ingestion
	// paused. Every other conversion still copies.
	get isAttachConversion(): boolean {
		return this.selectedTable === 'event_deliveries' && this.selectedOperation === 'partition';
	}

	// Unpartitioning avoids the copy only for a table that was converted by
	// attaching, because only that one has an original table to hand back. The
	// server reports which those are; an unknown state reads as false here, which
	// keeps the maintenance warning. The two mistakes are not symmetric: a pause
	// that was not needed costs a maintenance window, while telling an operator
	// not to pause when the run does drop writes loses acknowledged webhooks.
	get isDetachConversion(): boolean {
		return this.selectedOperation === 'unpartition' && !!this.stateOf(this.selectedTable)?.adopted;
	}

	get keepsIngestionRunning(): boolean {
		return this.isAttachConversion || this.isDetachConversion;
	}

	async ngOnInit() {
		// Refresh before reading, or a licensed instance shows the unlicensed gate
		// until the next full reload, because the cache is filled asynchronously by
		// the shell.
		await this.licenseService.loadAllLicenses();

		// Instance license, not the org intersection: the server gates this on the
		// deployment licenser alone, same as the CLI, and partitioning is an
		// instance-wide operation like queue monitoring.
		this.hasRetentionLicense = this.licenseService.hasInstanceLicense('RetentionPolicy');
		if (!this.hasRetentionLicense) return;

		await this.loadTableStates();
		await this.loadRuns();
	}

	async loadTableStates() {
		try {
			const response = await this.adminService.listPartitionTables();
			this.tableStates = response.data ?? [];
		} catch {
			// Fail open: without states the form offers both operations, and the
			// server, which reads the table's shape at the decision, is the one that
			// refuses an operation that cannot change it.
			this.tableStates = [];
		}

		this.applyOperations();
	}

	private stateOf(table: string): PartitionTable | undefined {
		return this.tableStates.find(state => state.name === table);
	}

	private applyOperations() {
		const state = this.stateOf(this.selectedTable);
		this.operations = state ? [state.partitioned ? UNPARTITION : PARTITION] : ALL_OPERATIONS;

		if (!this.operations.some(operation => operation.uid === this.selectedOperation)) {
			this.partitionForm.patchValue({ operation: this.operations[0].uid });
		}
	}

	// The sentence under the selects: what the table is now, and what the
	// operation would do to it.
	get operationHint(): string {
		const state = this.stateOf(this.selectedTable);
		if (!state) return '';

		return state.partitioned ? `${this.selectedTable} is partitioned. This converts it back to a plain table.` : `${this.selectedTable} is not partitioned. This converts it to a partitioned table.`;
	}

	ngOnDestroy() {
		this.destroyed = true;
		this.stopPolling();
		this.tableChanges.unsubscribe();
	}

	get activeRun(): PartitionRun | undefined {
		return this.runs.find(run => run.status === 'running');
	}

	get canStart(): boolean {
		const confirmation: string = this.partitionForm.value.confirmation ?? '';
		return !this.isStarting && !this.activeRun && confirmation.trim() === this.selectedTable;
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
			if (this.activeRun) {
				this.startPolling();
			} else {
				this.stopPolling();
				// A conversion that just finished changed the shape the form gates on.
				if (wasRunning) await this.loadTableStates();
			}
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
			this.partitionForm.patchValue({ confirmation: '' });
			const outcome = this.selectedOperation === 'partition' ? `${this.selectedTable} to a partitioned table` : `${this.selectedTable} back to a plain table`;
			this.generalService.showNotification({ message: `Started converting ${outcome}`, style: 'success' });
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

	elapsed(run: PartitionRun): string {
		const end = run.completed_at ? new Date(run.completed_at) : new Date();
		const seconds = Math.max(0, Math.floor((end.getTime() - new Date(run.started_at).getTime()) / 1000));

		// A conversion that finished inside a second is a table small enough to
		// rewrite instantly, not a run that did nothing, which is what a bare 0s
		// reads as.
		if (seconds === 0) return 'under a second';

		const hours = Math.floor(seconds / 3600);
		const minutes = Math.floor((seconds % 3600) / 60);
		if (hours > 0) return `${hours}h ${minutes}m`;
		if (minutes > 0) return `${minutes}m ${seconds % 60}s`;
		return `${seconds}s`;
	}

	// A running conversion shows its steps as they arrive, because that is the
	// only way to tell a long phase from a hung one. A finished one keeps them
	// behind a toggle: the outcome is in the title, and four finished runs of
	// eleven steps each would bury the list.
	showsSteps(run: PartitionRun): boolean {
		return run.status === 'running' || this.expandedRuns.has(run.uid);
	}

	toggleSteps(run: PartitionRun) {
		if (!this.expandedRuns.delete(run.uid)) this.expandedRuns.add(run.uid);
	}

	stepCount(run: PartitionRun): number {
		return run.steps?.length ?? 0;
	}

	// A row in the list is a record of what happened to a table, so it reads as
	// one. "unpartition event_deliveries" is the request that was posted, not a
	// description of it, and the status chip beside this already carries the
	// state, so the verb only has to agree with it.
	runTitle(run: PartitionRun): string {
		const outcome = run.operation === 'partition' ? `${run.table_name} to a partitioned table` : `${run.table_name} back to a plain table`;

		if (run.status === 'running') return `Converting ${outcome}`;
		if (run.status === 'failed') return `Could not convert ${outcome}`;
		return `Converted ${outcome}`;
	}
}
