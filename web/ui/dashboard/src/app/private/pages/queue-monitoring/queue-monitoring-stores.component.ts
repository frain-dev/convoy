import { CommonModule } from '@angular/common';
import { ChangeDetectorRef, Component, OnDestroy, OnInit } from '@angular/core';
import { AdminService } from '../admin/admin.service';

export interface QueueStoreSnapshot {
	observed_at: string;
	queues: Array<{
		name: string;
		pending: number;
		processing: number;
		scheduled: number;
		retry: number;
		future: number;
		aggregating: number;
		archived: number;
		completed: number;
		unknown: number;
		paused: boolean;
		oldest_due_age_ms: number;
	}>;
}

export interface QueueStore {
	id: string;
	provider: 'redis' | 'postgres';
	role: 'active' | 'previous';
	connection: 'connected' | 'unknown';
	snapshot: QueueStoreSnapshot | null;
	visible: boolean;
	visibility_reasons: string[];
	actions: string[];
}

@Component({
	selector: 'convoy-queue-monitoring-stores',
	imports: [CommonModule],
	templateUrl: './queue-monitoring-stores.component.html'
})
export class QueueMonitoringStoresComponent implements OnInit, OnDestroy {
	stores: QueueStore[] = [];
	loading = false;
	loadError = false;
	private destroyed = false;
	private timer?: ReturnType<typeof setTimeout>;

	constructor(private readonly admin: AdminService, private readonly cdr: ChangeDetectorRef) {}

	ngOnInit(): void {
		void this.refresh();
	}

	ngOnDestroy(): void {
		this.destroyed = true;
		clearTimeout(this.timer);
	}

	get previous(): QueueStore[] {
		return this.stores.filter(store => store.role === 'previous' && store.visible);
	}

	providerLabel(store: QueueStore): string {
		return store.provider === 'redis' ? 'Redis' : 'PostgreSQL';
	}

	async refresh(): Promise<void> {
		if (this.loading || this.destroyed) return;
		clearTimeout(this.timer);
		this.loading = true;
		try {
			const response = await this.admin.getQueueStores();
			if (this.destroyed) return;
			if (!Array.isArray(response.data)) throw new Error('Queue inventory is unavailable');
			this.stores = (response.data as QueueStore[]).map(store => {
				// Keep the previous observation only for the same registered store.
				// A successful empty observation may hide it; a failed one may not.
				const prior = this.stores.find(old => old.id === store.id);
				return store.connection === 'unknown' && prior?.snapshot ? { ...store, snapshot: prior.snapshot } : store;
			});
			this.loadError = false;
		} catch {
			if (this.destroyed) return;
			this.loadError = true;
			this.stores = this.stores.map(store => ({ ...store, connection: 'unknown', visible: store.role === 'previous' || store.visible, visibility_reasons: ['inspection_failed'], actions: [] }));
		} finally {
			this.loading = false;
			if (!this.destroyed) {
				this.cdr.markForCheck();
				this.timer = setTimeout(() => void this.refresh(), 15000);
			}
		}
	}
}
