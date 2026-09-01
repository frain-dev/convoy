import { Component, OnDestroy, OnInit } from '@angular/core';
import { FormBuilder, FormGroup } from '@angular/forms';
import { Subscription } from 'rxjs';
import { AdminService } from '../admin.service';
import { GeneralService } from 'src/app/services/general/general.service';
import { LicensesService } from 'src/app/services/licenses/licenses.service';
import { MaintenanceRun } from '../runs/run.model';

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

	// Full maintenance history from the server. Display filters to conversions;
	// rebuilds stay on the indexes page. The unfiltered list still drives
	// activeRun / canStart because any run holds the instance-wide slot.
	runs: MaintenanceRun[] = [];
	tableStates: PartitionTable[] = [];
	// Distinguishes "fetch failed" from "fetch returned no rows". Clearing
	// tableStates on error must not make adopted read as false; that would show
	// the copy-unpartition warning for a table the server will detach.
	tableStatesKnown = false;
	// Same reason, for the run history: before the first response lands there is
	// no basis for saying nothing has ever run here, and none for reporting an
	// error either, so failure is set only by the catch.
	runsKnown = false;
	runsFailed = false;
	isStarting = false;
	isLoadingRuns = false;
	hasRetentionLicense = false;
	// Whether the license read has been attempted, not that it succeeded:
	// loadAllLicenses swallows transport errors, so a failed read falls back to
	// the cache and reads as unlicensed on a cold one. What it buys is that
	// "requires a license key" waits for the read, not the initial false.
	licenseKnown = false;

	private pollInterval: any;
	private tableChanges: Subscription;
	private destroyed = false;

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

	// Read off the controls, not partitionForm.value. The group's aggregate value
	// trails a child write, so a getter built on it can still name the table the
	// operator just switched away from. applyOperations runs in that window, off
	// the table's own valueChanges, and would narrow the operation for the
	// previous table: pick a converted table and the select still offers, and
	// starts, the operation that suited the heap you left.
	get selectedTable(): string {
		return this.partitionForm.get('table')!.value;
	}

	get selectedOperation(): PartitionOperation {
		return this.partitionForm.get('operation')!.value;
	}

	// The table's current shape, not the form or the select. Switching tables
	// can leave both of those on the previous operation.
	get resolvedOperation(): PartitionOperation {
		const state = this.stateOf(this.selectedTable);
		if (state) {
			return state.partitioned ? 'unpartition' : 'partition';
		}
		return this.selectedOperation;
	}

	get isAttachConversion(): boolean {
		return this.resolvedOperation === 'partition';
	}

	get isDetachConversion(): boolean {
		return this.resolvedOperation === 'unpartition' && !!this.stateOf(this.selectedTable)?.adopted;
	}

	get isCopyUnpartition(): boolean {
		if (this.resolvedOperation !== 'unpartition' || !this.tableStatesKnown) return false;
		const state = this.stateOf(this.selectedTable);
		// !adopted is also true after retention drops the attach-converted
		// _default: there is no heap to detach, so unpartition copies. The
		// banner describes that copy, not how the table was first converted.
		return !!state && state.partitioned && !state.adopted;
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
		this.licenseKnown = true;
		if (!this.hasRetentionLicense) return;

		await this.loadTableStates();
		await this.loadRuns();
	}

	async loadTableStates() {
		try {
			const response = await this.adminService.listPartitionTables();
			this.tableStates = response.data ?? [];
			this.tableStatesKnown = true;
		} catch {
			// Fail open on the form: both operations stay available, and the server
			// refuses one that cannot change the table. Do not treat the empty list
			// as adopted=false; that is unknown, not a definitive copy-unpartition.
			this.tableStates = [];
			this.tableStatesKnown = false;
		}

		this.applyOperations();
	}

	private stateOf(table: string): PartitionTable | undefined {
		return this.tableStates.find(state => state.name === table);
	}

	applyOperations() {
		const state = this.stateOf(this.selectedTable);
		const next = state ? [state.partitioned ? UNPARTITION : PARTITION] : ALL_OPERATIONS;

		// Once the table's shape is known only one operation applies, so move the
		// control onto it rather than leaving the select on a stale choice the
		// server would refuse.
		if (next.length === 1 && this.selectedOperation !== next[0].uid) {
			this.partitionForm.patchValue({ operation: next[0].uid });
		}

		this.operations = next;
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

	get activeRun(): MaintenanceRun | undefined {
		return this.runs.find(run => run.status === 'running');
	}

	// Only partition / unpartition rows. Rebuild history is the indexes page's.
	get conversionRuns(): MaintenanceRun[] {
		return this.runs.filter(run => run.operation === 'partition' || run.operation === 'unpartition');
	}

	// runsKnown is part of the gate, not decoration: the slot is instance-wide and a
	// rebuild or a CLI conversion can take it between polls, so "no run in flight"
	// is a claim only a read that answered can make. Without it a failed poll leaves
	// the stale empty list saying the slot is free and the form offers a start the
	// server refuses.
	get canStart(): boolean {
		const confirmation: string = this.partitionForm.value.confirmation ?? '';
		return !this.isStarting && this.runsKnown && !this.activeRun && confirmation.trim() === this.selectedTable;
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
				// A conversion that just finished changed the shape the form gates on.
				if (wasRunning) await this.loadTableStates();
			}
		} catch {
			// Leaving the list as it was is better than blanking a run an operator
			// is watching because one poll failed. What the list can no longer do is
			// answer for the present: an earlier success does not survive a later
			// failure, so the empty state stops claiming nothing has ever run here
			// and the start gate stops reading the stale list as a free slot.
			this.runsKnown = false;
			this.runsFailed = true;
		} finally {
			this.isLoadingRuns = false;
		}
	}

	async startRun() {
		if (!this.canStart) return;

		this.isStarting = true;
		try {
			await this.adminService.startPartitionRun({ table: this.selectedTable, operation: this.resolvedOperation });
			this.partitionForm.patchValue({ confirmation: '' });
			const outcome = this.resolvedOperation === 'partition' ? `${this.selectedTable} to a partitioned table` : `${this.selectedTable} back to a plain table`;
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

}
