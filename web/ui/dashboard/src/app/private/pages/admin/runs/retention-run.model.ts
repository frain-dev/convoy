export type RetentionRunStatus = 'running' | 'completed' | 'failed' | 'skipped';

export interface RetentionTableDropDetail {
	table: string;
	dropped_partitions: string[] | null;
	dropped_default: boolean;
	dropped_rows?: number;
	dropped_default_rows?: number;
	dropped_partition_rows?: Record<string, number>;
	maintain_error?: string | null;
	not_partitioned?: boolean;
}

export interface RetentionRun {
	uid: string;
	status: RetentionRunStatus;
	retention_period: string;
	details: RetentionTableDropDetail[];
	error: string | null;
	started_at: string;
	completed_at: string | null;
}
