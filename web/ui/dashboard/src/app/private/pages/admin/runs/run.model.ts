// One run row covers both kinds of long maintenance work the admin pages drive:
// converting a table, and rebuilding an index a migration dropped. They share the
// row because they share the instance-wide single-active slot on the server. Each
// page loads that history and shows only its own operation; the unfiltered list
// still answers whether the slot is free.
export type MaintenanceOperation = 'partition' | 'unpartition' | 'rebuild_index';

export interface RunStep {
	message: string;
	at: string;
}

export interface MaintenanceRun {
	uid: string;
	table_name: string;
	operation: MaintenanceOperation;

	// Set only on a rebuild, where the table alone does not say what ran.
	index_name: string | null;

	status: 'running' | 'completed' | 'failed';
	phase: string | null;
	steps: RunStep[] | null;
	error: string | null;
	triggered_by: string;
	started_at: string;
	updated_at: string;
	completed_at: string | null;
}
