import { Component, ElementRef, EventEmitter, Input, OnInit, OnDestroy, Output, ViewChild } from '@angular/core';
import { ActivatedRoute } from '@angular/router';
import { Location } from '@angular/common';
import { format } from 'date-fns';
import { EVENT_DELIVERY, EVENT_TYPE, FILTER_QUERY_PARAM } from 'src/app/models/event.model';
import { PAGINATION } from 'src/app/models/global.model';
import { HTTP_RESPONSE } from 'src/app/models/global.model';
import { GeneralService } from 'src/app/services/general/general.service';
import { EventsService } from '../events.service';
import { PrivateService } from 'src/app/private/private.service';
import { ProjectService } from '../../project.service';
import { ENDPOINT } from 'src/app/models/endpoint.model';

@Component({
    selector: 'app-event-deliveries',
    templateUrl: './event-deliveries.component.html',
    styleUrls: ['./event-deliveries.component.scss'],
    standalone: false
})
export class EventDeliveriesComponent implements OnInit, OnDestroy {
	@Output() pushEventDeliveries = new EventEmitter<any>();
	// When set by a parent (portal / project events), overrides the list API's
	// implicit 7-day window so the table matches the page's chart/stat range.
	@Input() dateRange?: { startDate: string; endDate: string };
	eventDeliveryStatuses = ['Success', 'Failure', 'Retry', 'Scheduled', 'Processing', 'Discarded'];
	fetchingCount = false;
	showBatchRetryModal = false;
	isloadingEventDeliveries = false;
	isRetrying = false;
	loadError = false;
	batchRetryCount!: number;
	displayedEventDeliveries: { date: string; content: EVENT_DELIVERY[] }[] = [];
	eventDeliveries?: { pagination: PAGINATION; content: EVENT_DELIVERY[] };
	@ViewChild('batchRetryDialog', { static: true }) dialog!: ElementRef<HTMLDialogElement>;
	portalToken = this.route.snapshot.queryParams?.token;
	queryParams: FILTER_QUERY_PARAM = {};
	getEventDeliveriesInterval: any;

	// Filter/toolbar state (2026 UI refresh; the old shared filter bar was replaced
	// with the inline filter group from the redesign).
	statusDraft: string[] = [];
	statusFilter: string[] = [];
	selectedEndpointData?: ENDPOINT;
	filterEndpoints: ENDPOINT[] = [];
	loadingFilterEndpoints = false;
	endpointSearchString = '';
	private endpointSearchTimeout: any;
	eventTypes: EVENT_TYPE[] = [];
	sortOrder: 'asc' | 'desc' | string = 'desc';
	searchString = '';
	enableTailMode = false;

	// Client-side page tracking for the "1-10 of 50" pagination label; the API is
	// cursor-based so the absolute position is derived from prev/next navigation.
	currentPage = 1;
	totalCount?: number;
	selectedDeliveries = new Set<string>();
	batchRetryParams?: FILTER_QUERY_PARAM;
	batchRetryDate = '';

	constructor(
		private generalService: GeneralService,
		private eventsService: EventsService,
		public route: ActivatedRoute,
		public projectService: ProjectService,
		public privateService: PrivateService,
		private _location: Location
	) {}

	ngOnInit() {
		this.getFiltersFromURL();
		if (this.dateRange?.startDate && this.dateRange?.endDate && !this.queryParams.startDate) {
			this.queryParams = { ...this.queryParams, ...this.dateRange };
		}
		this.refreshDeliveries(false);
		if (this.checkIfTailModeIsEnabled()) this.getEventDeliveriesAtInterval();
		this.getEventTypesForFilter();
		if (!this.portalToken) this.getEndpointsForFilter();
		if (this.queryParams.endpointId) this.getSelectedEndpointData();
	}

	ngOnDestroy() {
		clearInterval(this.getEventDeliveriesInterval);
		clearTimeout(this.endpointSearchTimeout);
	}

	get isOutgoingProject(): boolean {
		return this.projectService.activeProjectDetails?.type === 'outgoing';
	}

	get showEventTypeColumn(): boolean {
		return this.isOutgoingProject || !!this.portalToken;
	}

	getFiltersFromURL() {
		this.queryParams = { ...this.queryParams, ...this.route.snapshot.queryParams };
		this.statusFilter = this.queryParams.status ? JSON.parse(this.queryParams.status) : [];
		this.statusDraft = [...this.statusFilter];
		this.sortOrder = this.queryParams?.sort || 'desc';
	}

	formatPreciseTimestamp(value?: string): string {
		if (!value) return '';

		const date = new Date(value);
		if (Number.isNaN(date.getTime())) return value;

		return date.toISOString();
	}

	// ------- filters -------

	toggleStatusDraft(status: string) {
		this.statusDraft.includes(status) ? (this.statusDraft = this.statusDraft.filter(s => s !== status)) : this.statusDraft.push(status);
	}

	applyStatusFilter() {
		this.statusFilter = [...this.statusDraft];
		this.queryParams = this.generalService.addFilterToURL({ ...this.queryParams, status: this.statusFilter.length ? JSON.stringify(this.statusFilter) : '' });
		if (!this.statusFilter.length) delete this.queryParams.status;
		this.refreshDeliveries();
	}

	clearStatusFilter() {
		this.statusDraft = [];
		this.applyStatusFilter();
	}

	async getEndpointsForFilter(search = '') {
		this.loadingFilterEndpoints = true;
		try {
			const response = await this.privateService.getEndpoints({ q: search });
			this.filterEndpoints = response.data.content || [];
		} catch (error) {
			this.filterEndpoints = [];
		}
		this.loadingFilterEndpoints = false;
	}

	onEndpointSearch() {
		clearTimeout(this.endpointSearchTimeout);
		this.endpointSearchTimeout = setTimeout(() => this.getEndpointsForFilter(this.endpointSearchString.trim()), 400);
	}

	updateEndpointFilter(endpoint: ENDPOINT) {
		this.selectedEndpointData = endpoint;
		this.queryParams = this.generalService.addFilterToURL({ ...this.queryParams, endpointId: endpoint.uid });
		this.refreshDeliveries();
	}

	clearEndpointFilter() {
		this.selectedEndpointData = undefined;
		delete this.queryParams.endpointId;
		this.queryParams = this.generalService.addFilterToURL({ ...this.queryParams });
		this.refreshDeliveries();
	}

	setEventType(eventType?: string) {
		if (eventType) this.queryParams = this.generalService.addFilterToURL({ ...this.queryParams, eventType });
		else {
			delete this.queryParams.eventType;
			this.queryParams = this.generalService.addFilterToURL({ ...this.queryParams });
		}
		this.refreshDeliveries();
	}

	setSortOrder(order: 'asc' | 'desc') {
		this.sortOrder = order;
		this.queryParams = this.generalService.addFilterToURL({ ...this.queryParams, sort: order });
		this.refreshDeliveries();
	}

	// Called by the parent page when the summary date range changes; the range
	// scopes the delivery table too.
	applyDateFilter(dateRange?: { startDate: string; endDate: string }) {
		if (dateRange) this.queryParams = this.generalService.addFilterToURL({ ...this.queryParams, ...dateRange });
		else {
			delete this.queryParams.startDate;
			delete this.queryParams.endDate;
			this.queryParams = this.generalService.addFilterToURL({ ...this.queryParams });
		}
		this.refreshDeliveries();
	}

	clearAllFilters() {
		this.statusFilter = [];
		this.statusDraft = [];
		this.selectedEndpointData = undefined;
		this.searchString = '';
		this.endpointSearchString = '';

		const sort = this.queryParams?.sort;
		this.queryParams = sort ? { sort } : {};
		this._location.go(`${location.pathname}${this.portalToken ? `?token=${this.portalToken}` : ''}`);
		this.refreshDeliveries();
	}

	get hasActiveFilters(): boolean {
		const keys = Object.keys(this.queryParams || {}).filter(k => !['sort', 'token', 'next_page_cursor', 'prev_page_cursor', 'direction', 'showLoader'].includes(k));
		return keys.length > 0 || !!this.searchString.trim();
	}

	// ------- search (client-side over the loaded page) -------

	onSearch() {
		if (this.eventDeliveries?.content) this.displayedEventDeliveries = this.setEventDeliveriesContent(this.eventDeliveries.content);
	}

	private applySearch(eventDeliveriesData: EVENT_DELIVERY[]): EVENT_DELIVERY[] {
		const searchTerm = this.searchString.trim().toLowerCase();
		if (!searchTerm) return eventDeliveriesData;

		return eventDeliveriesData.filter(delivery =>
			[delivery.uid, delivery.event_metadata?.event_type, delivery.endpoint_metadata?.title, delivery.endpoint_metadata?.name, delivery.source_metadata?.name].some(value => value?.toLowerCase().includes(searchTerm))
		);
	}

	// ------- tail mode -------

	checkIfTailModeIsEnabled() {
		const tailModeConfig = localStorage.getItem('EVENTS_TAIL_MODE');
		this.enableTailMode = tailModeConfig ? JSON.parse(tailModeConfig) : false;

		return this.enableTailMode;
	}

	toggleTailMode(e?: any, status?: 'on' | 'off') {
		let tailModeConfig: boolean;
		if (status) tailModeConfig = status === 'on';
		else tailModeConfig = e.target.checked;

		this.enableTailMode = tailModeConfig;
		localStorage.setItem('EVENTS_TAIL_MODE', JSON.stringify(tailModeConfig));

		clearInterval(this.getEventDeliveriesInterval);
		if (tailModeConfig) this.getEventDeliveriesAtInterval();
	}

	getEventDeliveriesAtInterval() {
		this.getEventDeliveriesInterval = setInterval(() => {
			this.getEventDeliveries(this.queryParams);
		}, 5000);
	}

	// ------- data -------

	refreshDeliveries(resetPage = true) {
		if (resetPage) {
			this.currentPage = 1;
			delete this.queryParams.next_page_cursor;
			delete this.queryParams.prev_page_cursor;
			delete this.queryParams.direction;
		}

		this.getEventDeliveries({ ...this.queryParams, showLoader: true });
		this.refreshTotalCount();
	}

	async getEventDeliveries(requestDetails?: FILTER_QUERY_PARAM): Promise<HTTP_RESPONSE> {
		if (requestDetails?.showLoader) this.isloadingEventDeliveries = true;

		try {
			const eventDeliveriesResponse = await this.eventsService.getEventDeliveries(requestDetails);
			this.eventDeliveries = eventDeliveriesResponse.data;
			this.displayedEventDeliveries = this.setEventDeliveriesContent(eventDeliveriesResponse.data.content);

			this.loadError = false;
			this.isloadingEventDeliveries = false;
			return eventDeliveriesResponse;
		} catch (error: any) {
			this.loadError = true;
			this.isloadingEventDeliveries = false;
			return error;
		}
	}

	// The shared grouping service emits labels like "28 July, 2026"; the design uses "28 July 2026".
	groupDateLabel(date: string): string {
		return date.replace(',', '');
	}

	setEventDeliveriesContent(eventDeliveriesData: EVENT_DELIVERY[]): { date: string; content: EVENT_DELIVERY[] }[] {
		return this.generalService.setContentDisplayed(this.applySearch(eventDeliveriesData) as any, this.queryParams?.sort || 'desc');
	}

	async refreshTotalCount() {
		const countParams = this.queryParamsForCount(this.queryParams);

		try {
			const response = await this.eventsService.getRetryCount(countParams);
			this.totalCount = response.data.num;
		} catch (error) {
			this.totalCount = undefined;
		}
	}

	async getEventTypesForFilter() {
		if (!this.isOutgoingProject) return;
		try {
			const response = await this.privateService.getEventTypes();
			// Older backends return { event_types: [...] }, newer ones return the array directly.
			const types: EVENT_TYPE[] = Array.isArray(response.data) ? response.data : response.data?.event_types || [];
			this.eventTypes = types.filter(type => !type.deprecated_at);
		} catch (error) {}
	}

	async getSelectedEndpointData() {
		try {
			const response = await this.privateService.getEndpoints();
			this.selectedEndpointData = response.data.content.find((item: ENDPOINT) => item.uid === this.queryParams.endpointId);
		} catch (error) {}
	}

	// ------- pagination -------

	paginateEvents(direction: 'next' | 'prev') {
		const pagination = this.eventDeliveries?.pagination;
		if (!pagination) return;

		const cursor =
			direction === 'next' ? { next_page_cursor: pagination.next_page_cursor, prev_page_cursor: '', direction: 'next' as const } : { prev_page_cursor: pagination.prev_page_cursor, next_page_cursor: '', direction: 'prev' as const };

		this.queryParams = this.generalService.addFilterToURL({ ...this.queryParams, ...cursor });
		this.currentPage = Math.max(1, this.currentPage + (direction === 'next' ? 1 : -1));
		this.getEventDeliveries({ ...this.queryParams, showLoader: true });
	}

	get pageRangeLabel(): string {
		const contentLength = this.eventDeliveries?.content?.length || 0;
		if (!contentLength) return '0 events';

		const perPage = this.eventDeliveries?.pagination?.per_page || contentLength;
		const start = (this.currentPage - 1) * perPage + 1;
		const end = start + contentLength - 1;

		return this.totalCount !== undefined ? `${start}-${end} of ${this.totalCount}` : `${start}-${end}`;
	}

	// ------- selection -------

	isSelected(uid: string): boolean {
		return this.selectedDeliveries.has(uid);
	}

	toggleSelection(uid: string) {
		this.selectedDeliveries.has(uid) ? this.selectedDeliveries.delete(uid) : this.selectedDeliveries.add(uid);
	}

	get allPageSelected(): boolean {
		const content = this.eventDeliveries?.content || [];
		return content.length > 0 && content.every(delivery => this.selectedDeliveries.has(delivery.uid));
	}

	toggleSelectAll() {
		const content = this.eventDeliveries?.content || [];
		if (this.allPageSelected) content.forEach(delivery => this.selectedDeliveries.delete(delivery.uid));
		else content.forEach(delivery => this.selectedDeliveries.add(delivery.uid));
	}

	// ------- display helpers -------

	statusPillClass(status?: string): string {
		if (status === 'Success') return 'bg-success-a3 text-success-11';
		if (status === 'Failure') return 'bg-error-a3 text-error-11';
		return 'bg-new.surface-muted text-new.text-secondary';
	}

	canRetry(status?: string): boolean {
		return status === 'Success' || status === 'Failure' || status === 'Discarded';
	}

	copyDeliveryId(delivery: EVENT_DELIVERY, event: Event) {
		event.stopPropagation();
		navigator.clipboard?.writeText(delivery.uid).then(() => {
			this.generalService.showNotification({ message: 'Delivery ID copied to clipboard', style: 'info' });
		});
	}

	// ------- retries -------

	async retryEvent(requestDetails: { e: any; eventDeliveryId: string }) {
		requestDetails.e.stopPropagation();

		try {
			const response = await this.eventsService.retryEvent({ eventId: requestDetails.eventDeliveryId });
			this.generalService.showNotification({ message: response.message, style: 'success' });
			return;
		} catch (error) {
			return error;
		}
	}

	// force retry successful events
	async forceRetryEvent(requestDetails: { e: any; eventDeliveryId: string }) {
		requestDetails.e.stopPropagation();
		const payload = {
			ids: [requestDetails.eventDeliveryId]
		};

		try {
			const response = await this.eventsService.forceRetryEvent({ body: payload });
			this.generalService.showNotification({ message: response.message, style: 'success' });
			return;
		} catch (error) {
			return error;
		}
	}

	// "Retry All" for a single date group: batch retry scoped to that calendar day
	// combined with the currently active filters.
	async openGroupRetry(groupDate: string, event: Event) {
		event.stopPropagation();

		const day = new Date(groupDate);
		if (Number.isNaN(day.getTime())) return;

		const filters = this.queryParamsForCount(this.queryParams);
		this.batchRetryParams = { ...filters, startDate: `${format(day, 'yyyy-MM-dd')}T00:00:00`, endDate: `${format(day, 'yyyy-MM-dd')}T23:59:59` };
		this.batchRetryDate = groupDate;

		this.fetchingCount = true;
		try {
			const response = await this.eventsService.getRetryCount(this.batchRetryParams);
			this.batchRetryCount = response.data.num;
			this.fetchingCount = false;
			this.dialog.nativeElement.showModal();
		} catch (error) {
			this.fetchingCount = false;
		}
	}

	async batchRetryEvent() {
		if (!this.batchRetryParams) return;
		this.isRetrying = true;

		try {
			const response = await this.eventsService.batchRetryEvent(this.batchRetryParams);

			this.generalService.showNotification({ message: response.message, style: 'success' });
			this.dialog.nativeElement.close();
			this.isRetrying = false;
			return;
		} catch (error) {
			this.isRetrying = false;
			return error;
		}
	}

	// Strip pagination/UI-only keys so count and batch-retry share the same filter surface.
	private queryParamsForCount(params: FILTER_QUERY_PARAM | undefined): FILTER_QUERY_PARAM {
		const { next_page_cursor: _next, prev_page_cursor: _prev, direction: _direction, sort: _sort, showLoader: _showLoader, ...filters } = params || {};
		return filters;
	}
}
