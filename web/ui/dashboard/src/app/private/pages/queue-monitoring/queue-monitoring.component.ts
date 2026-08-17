import { CommonModule } from '@angular/common';
import { Component, OnInit } from '@angular/core';
import { Router } from '@angular/router';

import { AdminService } from 'src/app/private/pages/admin/admin.service';
import { GeneralService } from 'src/app/services/general/general.service';
import { LicensesService } from 'src/app/services/licenses/licenses.service';
import { RbacService } from 'src/app/services/rbac/rbac.service';
import { TagComponent } from 'src/app/components/tag/tag.component';

type TaskAction = 'retry' | 'archive';

interface QueueStat {
	queue: string;
	counts: { [status: string]: number };
}

interface QueueStats {
	provider: string;
	statuses: string[];
	queues: QueueStat[];
}

interface QueueTask {
	id: string;
	queue: string;
	task_name: string;
	status: string;
	retry_count: number;
	max_retry: number;
	next_run_at?: string;
	claimed_at?: string;
	last_error?: string;
	created_at?: string;
	actions: TaskAction[];
}

@Component({
	selector: 'convoy-queue-monitoring',
	imports: [CommonModule, TagComponent],
	templateUrl: './queue-monitoring.component.html'
})
export class QueueMonitoringComponent implements OnInit {
	licensed = false;

	stats: QueueStats | null = null;
	isLoadingStats = false;
	statsError = '';

	// Set together: the drill-down is one queue in one status, and both come
	// from a row the operator clicked rather than from free-form input.
	selectedQueue = '';
	selectedStatus = '';
	tasks: QueueTask[] = [];
	page = 1;
	hasNext = false;
	isLoadingTasks = false;
	tasksError = '';
	runningAction = '';
	expandedTask = '';

	// Only the newest task request may write the list. Paging fast, or clicking
	// a second status while the first is in flight, otherwise leaves whichever
	// response lands last on screen.
	private taskRequest = 0;

	constructor(
		private readonly adminService: AdminService,
		private readonly generalService: GeneralService,
		private readonly rbacService: RbacService,
		private readonly router: Router,
		private readonly licenses: LicensesService
	) {}

	async ngOnInit(): Promise<void> {
		const role = await this.rbacService.getUserRole();
		if (role !== 'INSTANCE_ADMIN') {
			this.router.navigateByUrl('/');
			return;
		}

		// Refresh before reading, or a licensed instance shows the unlicensed
		// gate until the next full reload, because the shell fills the cache
		// asynchronously.
		await this.licenses.loadAllLicenses();
		this.licensed = this.licenses.hasInstanceLicense('AsynqMonitoring');
		if (!this.licensed) return;

		await this.loadStats();
	}

	get isDrilledDown(): boolean {
		return !!this.selectedQueue;
	}

	// Clicking the queue name lands on the status the provider lists first,
	// which is pending on both, rather than requiring the operator to aim at a
	// count cell.
	get defaultStatus(): string {
		return this.stats?.statuses[0] ?? '';
	}

	get providerLabel(): string {
		return this.stats?.provider === 'postgres' ? 'Postgres' : 'Redis';
	}

	async loadStats(): Promise<void> {
		this.isLoadingStats = true;
		this.statsError = '';

		try {
			const response = await this.adminService.getQueueStats();
			this.stats = response.data;
		} catch (error) {
			// Keep the last good stats on screen: an empty table would read as
			// "no queues", which is a different fact from "the read failed".
			this.statsError = typeof error === 'string' ? error : 'Could not load queue stats.';
		} finally {
			this.isLoadingStats = false;
		}
	}

	statusCount(queue: QueueStat, status: string): number {
		return queue.counts?.[status] ?? 0;
	}

	queueTotal(queue: QueueStat): number {
		return Object.values(queue.counts ?? {}).reduce((total, count) => total + count, 0);
	}

	async openTasks(queue: string, status: string): Promise<void> {
		this.selectedQueue = queue;
		this.selectedStatus = status;
		this.page = 1;
		await this.loadTasks();
	}

	closeTasks(): void {
		this.selectedQueue = '';
		this.selectedStatus = '';
		this.tasks = [];
		this.tasksError = '';
		this.hasNext = false;
		this.expandedTask = '';
		// Counts moved while the drill-down was open, and the operator is
		// looking at the landing view again.
		this.loadStats();
	}

	async changeStatus(status: string): Promise<void> {
		if (status === this.selectedStatus) return;
		this.selectedStatus = status;
		this.page = 1;
		await this.loadTasks();
	}

	async paginate(direction: 'prev' | 'next'): Promise<void> {
		const next = direction === 'next' ? this.page + 1 : this.page - 1;
		if (next < 1) return;
		this.page = next;
		await this.loadTasks();
	}

	async loadTasks(): Promise<void> {
		const request = ++this.taskRequest;
		this.isLoadingTasks = true;
		this.tasksError = '';
		// The row it belonged to is about to be replaced, and an id that survives
		// into the next page would open a different task's detail.
		this.expandedTask = '';

		try {
			const response = await this.adminService.getQueueTasks(this.selectedQueue, this.selectedStatus, this.page);
			if (request !== this.taskRequest) return;
			this.tasks = response.data?.tasks ?? [];
			this.hasNext = !!response.data?.has_next;
		} catch (error) {
			if (request !== this.taskRequest) return;
			this.tasks = [];
			this.hasNext = false;
			this.tasksError = typeof error === 'string' ? error : 'Could not load tasks.';
		} finally {
			if (request === this.taskRequest) this.isLoadingTasks = false;
		}
	}

	toggleTask(task: QueueTask): void {
		this.expandedTask = this.isExpanded(task) ? '' : task.id;
	}

	isExpanded(task: QueueTask): boolean {
		return this.expandedTask === task.id;
	}

	canRun(task: QueueTask, action: TaskAction): boolean {
		// The provider says which transitions it will take for this task, so the
		// page never offers a button the broker rejects.
		return task.actions.includes(action);
	}

	async runAction(task: QueueTask, action: TaskAction): Promise<void> {
		if (this.runningAction) return;
		this.runningAction = `${task.id}:${action}`;

		try {
			await this.adminService.runQueueTaskAction(task.queue, task.id, action);
			this.generalService.showNotification({ message: action === 'retry' ? 'Task queued for retry' : 'Task archived', style: 'success' });
			// The task left the status being listed, so the page it was on has
			// changed under it; both the list and the counts are re-read.
			await this.loadTasks();
			await this.loadStats();
		} catch {
			// The request interceptor already toasted the server's message.
		} finally {
			this.runningAction = '';
		}
	}

	isRunning(task: QueueTask, action: TaskAction): boolean {
		return this.runningAction === `${task.id}:${action}`;
	}

	retriesLabel(task: QueueTask): string {
		return `${task.retry_count}/${task.max_retry}`;
	}

	// Task ids share a long prefix (`single:<batch>:<task>`), so trimming the end
	// renders a column of identical-looking rows. The tail is what tells two
	// tasks apart, and the full id is on the tooltip and the copy button.
	taskIdLabel(task: QueueTask): string {
		const visible = 28;
		return task.id.length > visible ? `…${task.id.slice(-visible)}` : task.id;
	}

	copyTaskId(task: QueueTask): void {
		navigator.clipboard?.writeText(task.id).then(() => {
			this.generalService.showNotification({ message: 'Task ID copied to clipboard', style: 'info' });
		});
	}

	statusColor(status: string): 'primary' | 'error' | 'success' | 'warning' | 'neutral' {
		switch (status) {
			case 'processing':
				return 'primary';
			case 'archived':
				return 'error';
			case 'retry':
				return 'warning';
			case 'scheduled':
				return 'neutral';
			default:
				return 'success';
		}
	}
}
