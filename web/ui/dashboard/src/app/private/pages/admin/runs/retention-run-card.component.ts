import { Component, Input } from '@angular/core';
import { RetentionRun, RetentionTableDropDetail } from './retention-run.model';

@Component({
	selector: 'app-retention-run-card',
	templateUrl: './retention-run-card.component.html',
	standalone: false
})
export class RetentionRunCardComponent {
	@Input({ required: true }) run!: RetentionRun;

	private expanded = false;

	get showsDetails(): boolean {
		return this.run.status === 'running' || this.run.status === 'skipped' || this.expanded;
	}

	toggleDetails() {
		this.expanded = !this.expanded;
	}

	get title(): string {
		const window = this.run.retention_period;
		switch (this.run.status) {
			case 'running':
				return `Retention drop in progress (${window})`;
			case 'failed':
				return `Retention drop failed (${window})`;
			case 'skipped':
				return `Retention skipped (${window})`;
			default:
				return `Retention drop completed (${window})`;
		}
	}

	get elapsed(): string {
		const end = this.run.completed_at ? new Date(this.run.completed_at) : new Date();
		const seconds = Math.max(0, Math.floor((end.getTime() - new Date(this.run.started_at).getTime()) / 1000));
		if (seconds === 0) return 'under a second';
		const minutes = Math.floor(seconds / 60);
		if (minutes > 0) return `${minutes}m ${seconds % 60}s`;
		return `${seconds}s`;
	}

	partitionsDropped(table: RetentionTableDropDetail): string[] {
		return table.dropped_partitions ?? [];
	}

	totalDropped(table: RetentionTableDropDetail): number {
		return this.partitionsDropped(table).length + (table.dropped_default ? 1 : 0);
	}

	runTotalDropped(): number {
		return (this.run.details ?? []).reduce((sum, table) => sum + this.totalDropped(table), 0);
	}

	tableRowsDropped(table: RetentionTableDropDetail): number {
		return table.dropped_rows ?? 0;
	}

	hasPartitionRowCount(table: RetentionTableDropDetail, name: string): boolean {
		return table.dropped_partition_rows != null && name in table.dropped_partition_rows;
	}

	partitionRowsDropped(table: RetentionTableDropDetail, name: string): number {
		return table.dropped_partition_rows?.[name] ?? 0;
	}

	runTotalRowsDropped(): number {
		return (this.run.details ?? []).reduce((sum, table) => sum + this.tableRowsDropped(table), 0);
	}

	formatCount(value: number): string {
		return value.toLocaleString();
	}

	isTableNotPartitioned(table: RetentionTableDropDetail): boolean {
		if (this.run.status !== 'skipped') {
			return false;
		}
		if (table.not_partitioned) {
			return true;
		}
		// Older skipped rows listed only missing tables and omitted the flag.
		return this.skippedUsesLegacyDetails();
	}

	skippedUsesLegacyDetails(): boolean {
		if (this.run.status !== 'skipped') {
			return false;
		}
		return !(this.run.details ?? []).some(table => table.not_partitioned);
	}

	notPartitionedCount(): number {
		if (this.run.status !== 'skipped') {
			return 0;
		}
		const details = this.run.details ?? [];
		if (details.some(table => table.not_partitioned)) {
			return details.filter(table => table.not_partitioned).length;
		}
		return details.length;
	}
}
