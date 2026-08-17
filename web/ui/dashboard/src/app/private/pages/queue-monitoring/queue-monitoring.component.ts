import { CommonModule } from '@angular/common';
import { Component, HostBinding, HostListener, OnDestroy, OnInit } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { ActivatedRoute, Router } from '@angular/router';

import { AdminService, QueueTaskAction } from 'src/app/private/pages/admin/admin.service';
import { GeneralService } from 'src/app/services/general/general.service';
import { LicensesService } from 'src/app/services/licenses/licenses.service';
import { RbacService } from 'src/app/services/rbac/rbac.service';
import { TagComponent } from 'src/app/components/tag/tag.component';

type TaskAction = QueueTaskAction;

interface QueueStat {
	queue: string;
	counts: { [status: string]: number };
	paused: boolean;
	latency_ms: number;
	processed: number;
	failed: number;
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

interface HistoryPoint {
	date: string;
	processed: number;
	failed: number;
}

interface SchedulerEntry {
	id: string;
	spec: string;
	task_name: string;
	queue: string;
	next_run_at?: string;
	prev_run_at?: string;
}

// The two charts are a handful of divs each, already scaled by the component.
// A charting dependency for two bar charts would be more code to keep current
// than the bars themselves. They keep separate shapes because they plot
// different things, and one bar naming its value "processed" when it is a
// backlog depth would mislead the next reader of the template.
interface SizeBar {
	label: string;
	title: string;
	queued: number;
	height: number;
}

interface HistoryBar {
	label: string;
	title: string;
	processed: number;
	processedHeight: number;
	failed: number;
	failedHeight: number;
}

@Component({
	selector: 'convoy-queue-monitoring',
	imports: [CommonModule, FormsModule, TagComponent],
	templateUrl: './queue-monitoring.component.html'
})
export class QueueMonitoringComponent implements OnInit, OnDestroy {
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
	perPage = 50;
	perPageOptions = [25, 50, 100];
	hasNext = false;
	isLoadingTasks = false;
	tasksError = '';
	runningAction = '';
	expandedTask = '';

	// The search box is bound to searchInput; search is what was submitted.
	// Keeping them apart means the list does not re-query on every keystroke,
	// and the "clear" affordance can tell an active search from typing.
	searchInput = '';
	search = '';

	selected = new Set<string>();
	isRunningBulk = false;

	history: HistoryPoint[] = [];
	historyDays = 7;
	isLoadingHistory = false;

	schedulerEntries: SchedulerEntry[] = [];
	showScheduler = false;
	isLoadingScheduler = false;

	autoRefresh = false;
	lastUpdated: Date | null = null;
	private refreshTimer: ReturnType<typeof setInterval> | null = null;
	private isRefreshing = false;

	// Full screen is this page escaping the admin shell, not the admin shell
	// getting wider: the task table wants room the other admin tabs do not, and
	// widening the shell for all of them moves the sidebar when tabs change.
	// The host element becomes the overlay, so no state is torn down and
	// nothing refetches on the way in or out.
	fullScreen = false;

	@HostBinding('class') get hostClass(): string {
		return this.fullScreen ? 'block fixed inset-0 z-50 overflow-y-auto bg-white-100 p-24px' : 'block';
	}

	@HostListener('document:keydown.escape')
	exitFullScreen(): void {
		if (this.fullScreen) this.setFullScreen(false);
	}

	toggleFullScreen(): void {
		this.setFullScreen(!this.fullScreen);
	}

	// Full screen lives in the URL so a reload comes back the way the operator
	// left it, and so the wide view can be shared as a link. Merging keeps the
	// admin tab param, and a null clears the flag rather than leaving
	// `full=0` behind.
	private setFullScreen(on: boolean): void {
		this.fullScreen = on;
		this.router.navigate([], {
			relativeTo: this.route,
			queryParams: { queueFullScreen: on ? '1' : null },
			queryParamsHandling: 'merge',
			replaceUrl: true
		});
	}

	// Only the newest task request may write the list. Paging fast, or clicking
	// a second status while the first is in flight, otherwise leaves whichever
	// response lands last on screen.
	private taskRequest = 0;
	private historyRequest = 0;

	constructor(
		private readonly adminService: AdminService,
		private readonly generalService: GeneralService,
		private readonly rbacService: RbacService,
		private readonly router: Router,
		private readonly route: ActivatedRoute,
		private readonly licenses: LicensesService
	) {}

	async ngOnInit(): Promise<void> {
		this.fullScreen = this.route.snapshot.queryParams?.['queueFullScreen'] === '1';

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

	ngOnDestroy(): void {
		this.stopAutoRefresh();
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

	get selectedQueueStat(): QueueStat | null {
		return this.stats?.queues.find(q => q.queue === this.selectedQueue) ?? null;
	}

	get isBusy(): boolean {
		return this.isLoadingStats || this.isLoadingTasks;
	}

	// Actionable tasks in the current selection. A selection that includes rows
	// the broker will refuse would report failures the operator could have been
	// spared, so the bulk buttons offer only actions every selected row accepts.
	get selectedTasks(): QueueTask[] {
		return this.tasks.filter(task => this.selected.has(task.id));
	}

	get bulkActions(): TaskAction[] {
		const chosen = this.selectedTasks;
		if (!chosen.length) return [];
		return (['retry', 'run', 'archive', 'delete'] as TaskAction[]).filter(action => chosen.every(task => task.actions.includes(action)));
	}

	get allSelected(): boolean {
		return this.tasks.length > 0 && this.tasks.every(task => this.selected.has(task.id));
	}

	// A chart of nothing is worse than no chart: the history endpoint fills
	// every day in the window, so an idle queue still returns rows and would
	// otherwise draw an empty plot with a date axis under it. Both charts ask
	// whether there is anything to plot, not whether there is a response.
	get hasQueueDepth(): boolean {
		return (this.stats?.queues ?? []).some(queue => this.queueTotal(queue) > 0);
	}

	get hasHistory(): boolean {
		const totals = this.historyTotals;
		return totals.processed > 0 || totals.failed > 0;
	}

	// A background read is one the operator did not ask for, so it may not move
	// anything but the data: no spinner, no disabled buttons, no collapsing the
	// row they are reading. Only an explicit Refresh or a navigation shows work
	// happening.
	async loadStats(background = false): Promise<void> {
		if (!background) this.isLoadingStats = true;
		this.statsError = '';

		try {
			const response = await this.adminService.getQueueStats();
			this.stats = response.data;
			this.lastUpdated = new Date();
		} catch (error) {
			// Keep the last good stats on screen: an empty table would read as
			// "no queues", which is a different fact from "the read failed".
			this.statsError = typeof error === 'string' ? error : 'Could not load queue stats.';
		} finally {
			if (!background) this.isLoadingStats = false;
		}
	}

	statusCount(queue: QueueStat, status: string): number {
		return queue.counts?.[status] ?? 0;
	}

	queueTotal(queue: QueueStat): number {
		return Object.values(queue.counts ?? {}).reduce((total, count) => total + count, 0);
	}

	// Latency is only meaningful as "how far behind", so a queue that is keeping
	// up reads as up to date rather than as 0ms.
	latencyLabel(queue: QueueStat): string {
		const ms = queue.latency_ms ?? 0;
		if (ms < 1000) return 'Up to date';
		const seconds = Math.round(ms / 1000);
		if (seconds < 60) return `${seconds}s behind`;
		const minutes = Math.round(seconds / 60);
		if (minutes < 60) return `${minutes}m behind`;
		return `${Math.round(minutes / 60)}h behind`;
	}

	errorRate(queue: QueueStat): number {
		const total = (queue.processed ?? 0) + (queue.failed ?? 0);
		return total ? Math.round(((queue.failed ?? 0) / total) * 100) : 0;
	}

	// The queue-size chart is the landing view's shape: which queue is holding
	// the work, at a glance, without reading five count columns per row.
	get queueSizeBars(): SizeBar[] {
		const queues = this.stats?.queues ?? [];
		const totals = queues.map(queue => this.queueTotal(queue));
		const peak = Math.max(...totals, 1);

		return queues.map((queue, index) => ({
			label: queue.queue,
			title: `${queue.queue}: ${totals[index]} queued`,
			queued: totals[index],
			height: this.barHeight(totals[index], peak)
		}));
	}

	// Both series are scaled against one peak so the two colours stay
	// comparable: a failure bar next to a processed bar has to mean the same
	// unit of work, or the chart flatters a bad day.
	get historyBars(): HistoryBar[] {
		const peak = Math.max(...this.history.flatMap(point => [point.processed, point.failed]), 1);

		return this.history.map(point => ({
			label: this.dayLabel(point.date),
			title: `${point.date}: ${point.processed} processed, ${point.failed} failed`,
			processedHeight: this.barHeight(point.processed, peak),
			failedHeight: this.barHeight(point.failed, peak),
			processed: point.processed,
			failed: point.failed
		}));
	}

	get historyTotals(): { processed: number; failed: number } {
		return this.history.reduce((totals, point) => ({ processed: totals.processed + point.processed, failed: totals.failed + point.failed }), { processed: 0, failed: 0 });
	}

	// A non-zero value always draws something. A bar rounded down to nothing
	// reads as "no work that day", which is the one thing the chart must not
	// say about a day that had some.
	private barHeight(value: number, peak: number): number {
		if (!value) return 0;
		return Math.max(Math.round((value / peak) * 100), 4);
	}

	private dayLabel(date: string): string {
		const parsed = new Date(`${date}T00:00:00Z`);
		return isNaN(parsed.getTime()) ? date : parsed.toLocaleDateString(undefined, { month: 'short', day: 'numeric', timeZone: 'UTC' });
	}

	async openTasks(queue: string, status: string): Promise<void> {
		this.selectedQueue = queue;
		this.selectedStatus = status;
		this.page = 1;
		this.clearSearch();
		await Promise.all([this.loadTasks(), this.loadHistory()]);
	}

	closeTasks(): void {
		this.selectedQueue = '';
		this.selectedStatus = '';
		this.tasks = [];
		this.tasksError = '';
		this.hasNext = false;
		this.expandedTask = '';
		this.history = [];
		this.selected.clear();
		this.clearSearch();
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

	async changePerPage(size: number): Promise<void> {
		if (size === this.perPage) return;
		this.perPage = size;
		// The old page number points into a different set of rows once the page
		// size changes, so paging restarts rather than landing somewhere the
		// operator did not choose.
		this.page = 1;
		await this.loadTasks();
	}

	async paginate(direction: 'prev' | 'next'): Promise<void> {
		const next = direction === 'next' ? this.page + 1 : this.page - 1;
		if (next < 1) return;
		this.page = next;
		await this.loadTasks();
	}

	async submitSearch(): Promise<void> {
		const term = this.searchInput.trim();
		if (term === this.search) return;
		this.search = term;
		this.page = 1;
		await this.loadTasks();
	}

	async clearSearchAndReload(): Promise<void> {
		if (!this.search && !this.searchInput) return;
		this.clearSearch();
		this.page = 1;
		await this.loadTasks();
	}

	private clearSearch(): void {
		this.search = '';
		this.searchInput = '';
	}

	async loadTasks(background = false): Promise<void> {
		const request = ++this.taskRequest;
		if (!background) {
			this.isLoadingTasks = true;
			this.expandedTask = '';
		}
		this.tasksError = '';

		try {
			const response = await this.adminService.getQueueTasks(this.selectedQueue, this.selectedStatus, this.page, this.perPage, this.search);
			if (request !== this.taskRequest) return;
			this.tasks = response.data?.tasks ?? [];
			this.hasNext = !!response.data?.has_next;
			this.pruneSelection();
			this.lastUpdated = new Date();
		} catch (error) {
			if (request !== this.taskRequest) return;
			this.tasksError = typeof error === 'string' ? error : 'Could not load tasks.';
			// A failed background tick leaves the last good page on screen. The
			// rows already drawn are still the best answer available, and
			// blanking them costs the operator the context to act on a
			// transient error.
			if (!background) {
				this.tasks = [];
				this.hasNext = false;
				this.selected.clear();
			}
		} finally {
			if (request === this.taskRequest && !background) this.isLoadingTasks = false;
		}
	}

	async loadHistory(): Promise<void> {
		const request = ++this.historyRequest;
		this.isLoadingHistory = true;

		try {
			const response = await this.adminService.getQueueHistory(this.selectedQueue, this.historyDays);
			if (request !== this.historyRequest) return;
			this.history = response.data ?? [];
		} catch {
			if (request !== this.historyRequest) return;
			// The chart is context for the task list, not the reason the
			// operator opened the page. A failed read hides it rather than
			// taking the drill-down down with it; the interceptor already
			// surfaced the server's message.
			this.history = [];
		} finally {
			if (request === this.historyRequest) this.isLoadingHistory = false;
		}
	}

	async changeHistoryDays(days: number): Promise<void> {
		if (days === this.historyDays) return;
		this.historyDays = days;
		await this.loadHistory();
	}

	async toggleScheduler(): Promise<void> {
		this.showScheduler = !this.showScheduler;
		if (!this.showScheduler || this.schedulerEntries.length) return;

		this.isLoadingScheduler = true;
		try {
			const response = await this.adminService.getQueueSchedulerEntries();
			this.schedulerEntries = response.data ?? [];
		} catch {
			this.schedulerEntries = [];
		} finally {
			this.isLoadingScheduler = false;
		}
	}

	async togglePause(queue: QueueStat, event: Event): Promise<void> {
		event.stopPropagation();
		if (this.isLoadingStats) return;

		try {
			await this.adminService.setQueuePaused(queue.queue, !queue.paused);
			this.generalService.showNotification({ message: queue.paused ? `${queue.queue} resumed` : `${queue.queue} paused`, style: 'success' });
			// Re-read rather than flipping the flag locally: pausing changes
			// what the workers do next, and the counts move with it.
			await this.loadStats();
		} catch {
			// The request interceptor already toasted the server's message.
		}
	}

	toggleSelection(task: QueueTask, event: Event): void {
		event.stopPropagation();
		if (this.selected.has(task.id)) this.selected.delete(task.id);
		else this.selected.add(task.id);
	}

	toggleSelectAll(): void {
		if (this.allSelected) this.selected.clear();
		else this.tasks.forEach(task => this.selected.add(task.id));
	}

	isSelected(task: QueueTask): boolean {
		return this.selected.has(task.id);
	}

	// Ids that are no longer on screen are dropped: the operator can only judge
	// a bulk action against rows they can see, and a page change would
	// otherwise carry a hidden selection into it.
	private pruneSelection(): void {
		const visible = new Set(this.tasks.map(task => task.id));
		this.selected.forEach(id => {
			if (!visible.has(id)) this.selected.delete(id);
		});
	}

	async runBulkAction(action: TaskAction): Promise<void> {
		const ids = this.selectedTasks.map(task => task.id);
		// A single-row action in flight is moving one of these same rows, so a
		// bulk run started on top of it would report against a stale selection.
		if (!ids.length || this.isRunningBulk || this.runningAction) return;

		this.isRunningBulk = true;
		try {
			const response = await this.adminService.runQueueBulkAction(this.selectedQueue, action, ids);
			const failures = Object.keys(response.data?.failures ?? {}).length;
			const succeeded = response.data?.succeeded ?? 0;
			this.generalService.showNotification({
				message: failures ? `${succeeded} task(s) moved, ${failures} refused` : `${succeeded} task(s) moved`,
				style: failures ? 'warning' : 'success'
			});
			this.selected.clear();
			await Promise.all([this.loadTasks(), this.loadStats()]);
		} catch {
			// The request interceptor already toasted the server's message.
		} finally {
			this.isRunningBulk = false;
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

	async runAction(task: QueueTask, action: TaskAction, event: Event): Promise<void> {
		event.stopPropagation();
		if (this.runningAction || this.isRunningBulk) return;
		this.runningAction = `${task.id}:${action}`;

		try {
			await this.adminService.runQueueTaskAction(task.queue, task.id, action);
			this.generalService.showNotification({ message: `Task ${this.actionPastTense(action)}`, style: 'success' });
			// The task left the status being listed, so the page it was on has
			// changed under it; both the list and the counts are re-read.
			await Promise.all([this.loadTasks(), this.loadStats()]);
		} catch {
			// The request interceptor already toasted the server's message.
		} finally {
			this.runningAction = '';
		}
	}

	isRunning(task: QueueTask, action: TaskAction): boolean {
		return this.runningAction === `${task.id}:${action}`;
	}

	actionLabel(action: TaskAction): string {
		switch (action) {
			case 'retry':
				return 'Retry';
			case 'run':
				return 'Run now';
			case 'archive':
				return 'Archive';
			default:
				return 'Delete';
		}
	}

	actionPastTense(action: TaskAction): string {
		switch (action) {
			case 'retry':
				return 'queued for retry';
			case 'run':
				return 'scheduled to run now';
			case 'archive':
				return 'archived';
			default:
				return 'deleted';
		}
	}

	toggleAutoRefresh(): void {
		this.autoRefresh = !this.autoRefresh;
		if (this.autoRefresh) this.startAutoRefresh();
		else this.stopAutoRefresh();
	}

	// Refreshing pulls whichever view is open. It skips a tick while a read is
	// already in flight so a slow broker cannot queue up requests, and it never
	// fires while an action is running, which would reload the list underneath
	// the operator mid-click. A hidden tab is not being read by anyone, so it
	// polls nothing until it comes back.
	private startAutoRefresh(): void {
		this.stopAutoRefresh();
		this.refreshTimer = setInterval(() => this.refreshTick(), 10000);
	}

	private async refreshTick(): Promise<void> {
		if (this.isRefreshing || this.isBusy || this.runningAction || this.isRunningBulk) return;
		if (document.hidden) return;

		this.isRefreshing = true;
		try {
			if (this.isDrilledDown) await this.loadTasks(true);
			else await this.loadStats(true);
		} finally {
			this.isRefreshing = false;
		}
	}

	private stopAutoRefresh(): void {
		if (!this.refreshTimer) return;
		clearInterval(this.refreshTimer);
		this.refreshTimer = null;
	}

	retriesLabel(task: QueueTask): string {
		return `${task.retry_count}/${task.max_retry}`;
	}

	// Task ids share a long prefix (`single:<batch>:<task>`), so trimming the end
	// renders a column of identical-looking rows. The tail is what tells two
	// tasks apart, and the full id is on the tooltip and the copy button.
	taskIdLabel(task: QueueTask): string {
		const visible = 24;
		return task.id.length > visible ? `…${task.id.slice(-visible)}` : task.id;
	}

	copyTaskId(task: QueueTask, event: Event): void {
		event.stopPropagation();
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
