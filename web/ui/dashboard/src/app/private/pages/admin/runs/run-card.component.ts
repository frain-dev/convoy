import { Component, Input } from '@angular/core';
import { MaintenanceRun } from './run.model';

// One run in the history, shared by the pages that start runs. Both show the same
// row from the same table, so the status, the step stream and the elapsed time are
// rendered once here rather than copied per page, where they would drift.
@Component({
	selector: 'app-admin-run-card',
	templateUrl: './run-card.component.html',
	standalone: false
})
export class RunCardComponent {
	@Input({ required: true }) run!: MaintenanceRun;

	private expanded = false;

	// A running operation shows its steps as they arrive, because that is the only
	// way to tell a long phase from a hung one. A finished one keeps them behind a
	// toggle: the outcome is in the title, and several finished runs of a dozen
	// steps each would bury the list.
	get showsSteps(): boolean {
		return this.run.status === 'running' || this.expanded;
	}

	toggleSteps() {
		this.expanded = !this.expanded;
	}

	get stepCount(): number {
		return this.run.steps?.length ?? 0;
	}

	get elapsed(): string {
		const end = this.run.completed_at ? new Date(this.run.completed_at) : new Date();
		const seconds = Math.max(0, Math.floor((end.getTime() - new Date(this.run.started_at).getTime()) / 1000));

		// Work that finished inside a second is a table small enough to rewrite
		// instantly, not a run that did nothing, which is what a bare 0s reads as.
		if (seconds === 0) return 'under a second';

		const hours = Math.floor(seconds / 3600);
		const minutes = Math.floor((seconds % 3600) / 60);
		if (hours > 0) return `${hours}h ${minutes}m`;
		if (minutes > 0) return `${minutes}m ${seconds % 60}s`;
		return `${seconds}s`;
	}

	// A row is a record of what happened, so it reads as one. "unpartition
	// event_deliveries" is the request that was posted, not a description of it,
	// and the status chip beside this already carries the state, so the verb only
	// has to agree with it.
	get title(): string {
		if (this.run.operation === 'rebuild_index') {
			// index_name is what the server sets on every rebuild row; the table is
			// the fallback only so a row can never render a bare verb.
			const subject = this.run.index_name ?? `an index on ${this.run.table_name}`;
			if (this.run.status === 'running') return `Rebuilding ${subject}`;
			if (this.run.status === 'failed') return `Could not rebuild ${subject}`;
			return `Rebuilt ${subject}`;
		}

		const outcome = this.run.operation === 'partition' ? `${this.run.table_name} to a partitioned table` : `${this.run.table_name} back to a plain table`;

		if (this.run.status === 'running') return `Converting ${outcome}`;
		if (this.run.status === 'failed') return `Could not convert ${outcome}`;
		return `Converted ${outcome}`;
	}
}
