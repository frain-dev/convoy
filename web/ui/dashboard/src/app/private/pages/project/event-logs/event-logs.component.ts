import { Component, ElementRef, OnInit, OnDestroy, ViewChild } from '@angular/core';
import { CommonModule } from '@angular/common';
import { PrivateService } from 'src/app/private/private.service';
import { ActivatedRoute, Router, RouterModule } from '@angular/router';
import { CURSOR, PAGINATION } from 'src/app/models/global.model';
import { EventLogsService } from './event-logs.service';
import { GeneralService } from 'src/app/services/general/general.service';
import { SOURCE } from 'src/app/models/source.model';
import { EVENT, EVENT_DELIVERY, FILTER_QUERY_PARAM } from 'src/app/models/event.model';
import { StatusColorModule } from 'src/app/pipes/status-color/status-color.module';
import { PrismModule } from 'src/app/private/components/prism/prism.module';
import { FormsModule } from '@angular/forms';
import { DialogDirective } from 'src/app/components/dialog/dialog.directive';
import { EventsService } from '../events/events.service';
import { TagComponent } from 'src/app/components/tag/tag.component';
import { DatePickerComponent } from 'src/app/components/date-picker/date-picker.component';
import { DropdownComponent, DropdownOptionDirective } from 'src/app/components/dropdown/dropdown.component';
import { LicensesService } from 'src/app/services/licenses/licenses.service';

@Component({
    selector: 'convoy-event-logs',
    imports: [
        CommonModule,
        RouterModule,
        FormsModule,
        StatusColorModule,
        PrismModule,
        TagComponent,
        DialogDirective,
        DatePickerComponent,
        DropdownComponent,
        DropdownOptionDirective
    ],
    templateUrl: './event-logs.component.html',
    styleUrls: ['./event-logs.component.scss']
})
export class EventLogsComponent implements OnInit, OnDestroy {
	@ViewChild('batchDialog', { static: true }) batchDialog!: ElementRef<HTMLDialogElement>;
	@ViewChild('datePicker') datePicker?: DatePickerComponent;

	isloadingEvents: boolean = false;
	fetchError = false;
	displayedEvents: { date: string; content: EVENT[] }[] = [];
	events?: { pagination: PAGINATION; content: EVENT[] };
	duplicateEvents!: EVENT[];
	eventsDetailsItem: any;
	sidebarEventDeliveries: EVENT_DELIVERY[] = [];
	portalToken = this.route.snapshot.params?.token;
	filterSources: SOURCE[] = [];
	selectedSourceData?: SOURCE;
	isLoadingSidebarDeliveries = true;
	fetchingCount = false;
	isRetrying = false;
	isFetchingDuplicateEvents = false;
	batchRetryCount: any;
	getEventsInterval: any;
	queryParams: FILTER_QUERY_PARAM = {};
	enableTailMode = false;
	sortOrder: 'asc' | 'desc' | string = 'desc';
	searchString = '';
	currentPage = 1;

	private searchTimeout: any;

	constructor(
		private eventsLogService: EventLogsService,
		public generalService: GeneralService,
		public route: ActivatedRoute,
		private router: Router,
		public privateService: PrivateService,
		private eventsService: EventsService,
		public licenseService: LicensesService
	) {}

	ngOnInit() {
		this.queryParams = { ...this.route.snapshot.queryParams };
		this.searchString = this.queryParams.query || '';
		this.sortOrder = this.queryParams.sort || 'desc';

		this.enableTailMode = this.checkIfTailModeIsEnabled();
		if (this.enableTailMode) this.getEventsAtInterval();

		if (this.privateService.getProjectDetails?.type === 'incoming') this.getSourcesForFilter();

		this.getEventLogs({ showLoader: true });
	}

	ngOnDestroy() {
		clearInterval(this.getEventsInterval);
		clearTimeout(this.searchTimeout);
	}

	get isIncomingProject(): boolean {
		return this.privateService.getProjectDetails?.type === 'incoming';
	}

	// ------- filters -------

	get hasActiveFilters(): boolean {
		const keys = Object.keys(this.queryParams || {}).filter(k => !['sort', 'token', 'next_page_cursor', 'prev_page_cursor', 'direction', 'showLoader'].includes(k));
		return keys.length > 0;
	}

	get dateRangeLabel(): string {
		if (!this.queryParams.startDate || !this.queryParams.endDate) return 'All time';
		const format = (value: string) => new Date(value).toLocaleDateString('en-GB', { day: '2-digit', month: 'short' });
		return `${format(this.queryParams.startDate)} - ${format(this.queryParams.endDate)}`;
	}

	refreshEvents(showLoader = false) {
		this.currentPage = 1;
		delete this.queryParams.next_page_cursor;
		delete this.queryParams.prev_page_cursor;
		delete this.queryParams.direction;
		this.getEventLogs({ showLoader });
	}

	onSearch() {
		clearTimeout(this.searchTimeout);
		this.searchTimeout = setTimeout(() => {
			this.queryParams = this.generalService.addFilterToURL({ ...this.queryParams, query: this.searchString.trim() });
			this.refreshEvents();
		}, 400);
	}

	getSelectedDateRange(dateRange?: { startDate: string; endDate: string }) {
		if (dateRange) {
			this.queryParams = this.generalService.addFilterToURL({ ...this.queryParams, ...dateRange });
		} else {
			this.queryParams = this.generalService.addFilterToURL({ ...this.queryParams, startDate: '', endDate: '' });
		}
		this.refreshEvents();
	}

	setSortOrder(order: 'asc' | 'desc') {
		this.sortOrder = order;
		this.queryParams = this.generalService.addFilterToURL({ ...this.queryParams, sort: order });
		this.refreshEvents();
	}

	async getSourcesForFilter() {
		try {
			const response = await this.privateService.getSources();
			this.filterSources = response.data.content || [];
			if (this.queryParams.sourceId && !this.selectedSourceData) {
				this.selectedSourceData = this.filterSources.find(source => source.uid === this.queryParams.sourceId);
			}
		} catch {
			this.filterSources = [];
		}
	}

	updateSourceFilter(source: SOURCE) {
		this.selectedSourceData = source;
		this.queryParams = this.generalService.addFilterToURL({ ...this.queryParams, sourceId: source.uid });
		this.refreshEvents();
	}

	clearSourceFilter() {
		this.selectedSourceData = undefined;
		this.queryParams = this.generalService.addFilterToURL({ ...this.queryParams, sourceId: '' });
		this.refreshEvents();
	}

	clearAllFilters() {
		this.searchString = '';
		this.selectedSourceData = undefined;
		this.datePicker?.clearDate();
		this.queryParams = this.generalService.addFilterToURL({ ...this.queryParams, query: '', sourceId: '', startDate: '', endDate: '', eventId: '', idempotencyKey: '' });
		this.refreshEvents();
	}

	// ------- tail mode -------

	toggleTailMode(event: any) {
		const tailModeConfig = event?.target ? event.target.checked : !!event;
		this.enableTailMode = tailModeConfig;
		localStorage.setItem('EVENT_LOGS_TAIL_MODE', JSON.stringify(tailModeConfig));

		clearInterval(this.getEventsInterval);
		if (tailModeConfig) this.getEventsAtInterval();
	}

	getEventsAtInterval() {
		this.getEventsInterval = setInterval(() => {
			this.getEventLogs();
		}, 5000);
	}

	checkIfTailModeIsEnabled() {
		const tailModeConfig = localStorage.getItem('EVENT_LOGS_TAIL_MODE');
		return tailModeConfig ? JSON.parse(tailModeConfig) : false;
	}

	// ------- data fetching -------

	async getEventLogs(requestDetails?: { showLoader?: boolean }) {
		if (requestDetails?.showLoader) this.isloadingEvents = true;
		this.fetchError = false;

		try {
			const cleanedQuery: any = JSON.parse(JSON.stringify(this.queryParams));
			delete cleanedQuery.showLoader;
			const eventsResponse = await this.eventsService.getEvents(cleanedQuery);
			this.events = eventsResponse.data;

			this.displayedEvents = await this.generalService.setContentDisplayed(eventsResponse.data.content, this.queryParams?.sort || 'desc');
			this.isloadingEvents = false;

			if (this.eventsDetailsItem) return;
			else {
				this.eventsDetailsItem = this.events?.content[0];
				if (this.eventsDetailsItem?.uid) {
					this.getEventDeliveriesForSidebar(this.eventsDetailsItem.uid);
					this.getDuplicateEvents(this.eventsDetailsItem);
				} else this.isLoadingSidebarDeliveries = false;
			}

			return eventsResponse;
		} catch (error: any) {
			this.isloadingEvents = false;
			this.fetchError = true;
			return error;
		}
	}

	async getDuplicateEvents(event: EVENT) {
		if (!event.is_duplicate_event || !event.idempotency_key) return;

		this.isFetchingDuplicateEvents = true;
		try {
			const eventsResponse = await this.eventsService.getEvents({
				idempotencyKey: event?.idempotency_key
			});
			this.duplicateEvents = eventsResponse.data.content;
			this.isFetchingDuplicateEvents = false;
		} catch {
			this.isFetchingDuplicateEvents = false;
		}
	}

	async getEventDeliveriesForSidebar(eventId: string) {
		this.isLoadingSidebarDeliveries = true;
		this.sidebarEventDeliveries = [];

		try {
			const response = await this.eventsService.getEventDeliveries({ eventId });
			this.sidebarEventDeliveries = response.data.content;
			this.isLoadingSidebarDeliveries = false;

			return;
		} catch (error) {
			this.isLoadingSidebarDeliveries = false;
			return error;
		}
	}

	// ------- pagination -------

	paginateEvents(direction: 'next' | 'prev') {
		const pagination = this.events?.pagination;
		if (!pagination) return;

		const cursor: CURSOR =
			direction === 'next' ? { next_page_cursor: pagination.next_page_cursor, prev_page_cursor: '', direction: 'next' } : { prev_page_cursor: pagination.prev_page_cursor, next_page_cursor: '', direction: 'prev' };

		this.currentPage = Math.max(1, this.currentPage + (direction === 'next' ? 1 : -1));
		this.queryParams = this.generalService.addFilterToURL({ ...this.queryParams, ...cursor });
		this.getEventLogs();
	}

	get pageRangeLabel(): string {
		const contentLength = this.events?.content?.length || 0;
		if (!contentLength) return '0 events';

		const perPage = this.events?.pagination?.per_page || contentLength;
		const start = (this.currentPage - 1) * perPage + 1;
		const end = start + contentLength - 1;
		const total = this.events?.pagination?.total;

		return total ? `${start}-${end} of ${total}` : `${start}-${end}`;
	}

	groupDateLabel(date: string): string {
		return date.replace(',', '');
	}

	// ------- retries -------

	async fetchRetryCount() {
		this.fetchingCount = true;
		try {
			const response = await this.eventsLogService.getRetryCount(this.queryParams);

			this.batchRetryCount = response.data.num;
			this.fetchingCount = false;
			this.batchDialog.nativeElement.showModal();
		} catch (error) {
			this.fetchingCount = false;
		}
	}

	async replayEvent(requestDetails: { eventId: string }) {
		this.isRetrying = true;
		try {
			const response = await this.eventsLogService.retryEvent({ eventId: requestDetails.eventId });
			this.generalService.showNotification({ message: response.message, style: 'success' });
			this.isRetrying = false;
			return;
		} catch (error) {
			this.isRetrying = false;
			return error;
		}
	}

	async batchReplayEvent() {
		this.isRetrying = true;

		try {
			const response = await this.eventsLogService.batchRetryEvent(this.queryParams);

			this.generalService.showNotification({ message: response.message, style: 'success' });
			this.batchDialog.nativeElement.close();
			this.isRetrying = false;
		} catch (error) {
			this.isRetrying = false;
		}
	}

	// ------- misc -------

	viewSource(sourceId?: string) {
		if (!sourceId || this.portalToken) return;
		this.router.navigate([`/projects/${this.privateService.getProjectDetails?.uid}/sources/${sourceId}`]);
	}

	viewEventDeliveries(event: EVENT, filterByIdempotencyKey?: boolean) {
		const queryParams: any = {
			eventId: event.uid
		};
		if (filterByIdempotencyKey) queryParams['idempotencyKey'] = event.idempotency_key;

		const url = this.router.serializeUrl(this.router.createUrlTree([`/projects/${this.privateService.getProjectDetails?.uid}/events`], { queryParams }));
		window.open(url, '_blank');
	}

	copyText(text: string | undefined, label: string, event: Event) {
		event.stopPropagation();
		if (!text) return;
		navigator.clipboard?.writeText(text).then(() => {
			this.generalService.showNotification({ message: `${label} copied to clipboard`, style: 'info' });
		});
	}

	hasMatchedSubscription(event?: EVENT): boolean {
		return !!event?.endpoints?.length;
	}

	getRouteStatus(event: EVENT): string {
		return this.hasMatchedSubscription(event) ? 'Delivered' : 'Unmatched';
	}
}
