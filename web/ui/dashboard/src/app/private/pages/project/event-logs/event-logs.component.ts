import { Component, ElementRef, OnInit, OnDestroy, ViewChild } from '@angular/core';
import { CommonModule } from '@angular/common';
import { DomSanitizer, SafeHtml } from '@angular/platform-browser';
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
import { TooltipComponent } from 'src/app/components/tooltip/tooltip.component';
import { ProjectService } from '../project.service';
import { classifyEventLogsFetchError } from './event-logs-fetch-error';

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
        DropdownOptionDirective,
        TooltipComponent
    ],
    templateUrl: './event-logs.component.html',
    styleUrls: ['./event-logs.component.scss']
})
export class EventLogsComponent implements OnInit, OnDestroy {
	@ViewChild('batchDialog', { static: true }) batchDialog!: ElementRef<HTMLDialogElement>;
	@ViewChild('datePicker') datePicker?: DatePickerComponent;

	isloadingEvents: boolean = false;
	isSearchingEvents = false;
	fetchError = false;
	fetchErrorMessage = '';
	searchTimedOut = false;
	displayedEvents: { date: string; content: EVENT[] }[] = [];
	events?: { pagination: PAGINATION; content: EVENT[] };
	duplicateEvents!: EVENT[];
	eventsDetailsItem: any;
	sidebarEventDeliveries: EVENT_DELIVERY[] = [];
	portalToken = this.route.snapshot.params?.token;
	filterSources: SOURCE[] = [];
	selectedSourceData?: SOURCE;
	isLoadingSidebarDeliveries = false;
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
	customDateRange = false;
	payloadAutoScrollKey = '';

	private searchTimeout: any;
	private eventsFetchId = 0;
	private sidebarFetchId = 0;
	private sidebarLoadedForEventId = '';
	private sidebarLoadedRange = '';

	constructor(
		private eventsLogService: EventLogsService,
		public generalService: GeneralService,
		public route: ActivatedRoute,
		private router: Router,
		public privateService: PrivateService,
		private eventsService: EventsService,
		public licenseService: LicensesService,
		private projectService: ProjectService,
		private sanitizer: DomSanitizer
	) {}

	ngOnInit() {
		this.syncFiltersFromRoute(this.route.snapshot.queryParams);
		this.enableTailMode = this.checkIfTailModeIsEnabled();
		if (this.enableTailMode) this.getEventsAtInterval();

		if (this.projectConfig?.type === 'incoming') this.getSourcesForFilter();

		this.getEventLogs({ showLoader: true });
	}

	ngOnDestroy() {
		clearInterval(this.getEventsInterval);
		clearTimeout(this.searchTimeout);
	}

	get isIncomingProject(): boolean {
		return this.projectConfig?.type === 'incoming';
	}

	get canSearchEvents(): boolean {
		return this.licenseService.hasLicense('EventSearch');
	}

	private syncFiltersFromRoute(params: FILTER_QUERY_PARAM) {
		this.queryParams = { ...params };
		const query = this.queryParamString(params.query);
		const body = this.queryParamString(params.body);
		this.searchString = query && body ? `${query} ${body}` : query || body || '';
		this.sortOrder = params.sort || 'desc';
		this.customDateRange = !!(params.startDate && params.endDate);
	}

	private queryParamString(value?: string | string[]): string {
		if (Array.isArray(value)) return value.join(',');
		return value || '';
	}

	get trimmedSearchInput(): string {
		return this.searchString.trim();
	}

	get hasActiveSearch(): boolean {
		return !!(this.queryParams.query || this.queryParams.body);
	}

	get activeMetadataSearchTerm(): string {
		const raw = (this.queryParams.query || '').trim();
		if (!raw || raw.startsWith('{')) return '';
		return raw;
	}

	highlightSearchText(value?: string): SafeHtml {
		const text = value || '';
		const term = this.activeMetadataSearchTerm;
		if (!term || !text) {
			return this.sanitizer.bypassSecurityTrustHtml(this.escapeHtml(text));
		}
		const parts = text.split(new RegExp(`(${this.escapeRegExp(term)})`, 'ig'));
		const html = parts
			.map(part => (part.toLowerCase() === term.toLowerCase() ? `<mark class="search-hit">${this.escapeHtml(part)}</mark>` : this.escapeHtml(part)))
			.join('');
		return this.sanitizer.bypassSecurityTrustHtml(html);
	}

	get payloadSearchHighlightKeys(): string[] {
		const keys: string[] = [];
		const obj = this.payloadSearchFilterObject();
		if (obj) {
			this.collectJsonSearchHighlightTerms(obj, keys, []);
		}
		return [...new Set(keys.filter(k => k.length > 0))].sort((a, b) => b.length - a.length);
	}

	get payloadSearchHighlightValues(): string[] {
		const values: string[] = [];
		const meta = this.activeMetadataSearchTerm;
		if (meta) values.push(meta);

		const obj = this.payloadSearchFilterObject();
		if (obj) {
			this.collectJsonSearchHighlightTerms(obj, [], values);
		}

		return [...new Set(values.filter(v => v.length > 0))].sort((a, b) => b.length - a.length);
	}

	get isJsonPayloadSearch(): boolean {
		return !!this.payloadSearchFilterObject();
	}

	private payloadSearchFilterObject(): Record<string, unknown> | null {
		const fromBody = this.queryParamString(this.queryParams.body).trim();
		if (fromBody) {
			try {
				const parsed = JSON.parse(fromBody);
				if (parsed !== null && typeof parsed === 'object' && !Array.isArray(parsed)) {
					return parsed as Record<string, unknown>;
				}
			} catch {
				// body is not JSON; fall through to query
			}
		}
		const raw = (this.queryParamString(this.queryParams.query) || this.searchString).trim();
		if (!raw) return null;
		return this.parseRelaxedJsonObject(raw);
	}

	private collectJsonSearchHighlightTerms(value: unknown, keys: string[], values: string[], depth = 0): void {
		if (depth > 12 || value == null) return;

		switch (typeof value) {
			case 'string':
			case 'number':
			case 'boolean':
				values.push(String(value));
				return;
			case 'object':
				if (Array.isArray(value)) {
					value.forEach(item => this.collectJsonSearchHighlightTerms(item, keys, values, depth + 1));
					return;
				}
				Object.entries(value as Record<string, unknown>).forEach(([key, nested]) => {
					keys.push(key);
					this.collectJsonSearchHighlightTerms(nested, keys, values, depth + 1);
				});
		}
	}

	private escapeHtml(value: string): string {
		return value
			.replace(/&/g, '&amp;')
			.replace(/</g, '&lt;')
			.replace(/>/g, '&gt;')
			.replace(/"/g, '&quot;')
			.replace(/'/g, '&#39;');
	}

	private escapeRegExp(value: string): string {
		return value.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
	}

	private updatePayloadAutoScrollKey(eventUid?: string) {
		if (!this.hasActiveSearch) {
			this.payloadAutoScrollKey = '';
			return;
		}

		const uid = eventUid || this.eventsDetailsItem?.uid;
		if (!uid) {
			return;
		}

		this.payloadAutoScrollKey = `${this.queryParams.query || ''}\0${this.queryParams.body || ''}\0${uid}`;
	}

	selectEvent(event: EVENT) {
		this.eventsDetailsItem = event;
		this.getEventDeliveriesForSidebar(event.uid);
		this.getDuplicateEvents(event);
		this.updatePayloadAutoScrollKey(event.uid);
	}

	private looksLikePayloadSearchInput(raw: string): boolean {
		const trimmed = raw.trim();
		if (!trimmed) return false;
		if (trimmed.startsWith('{')) return true;
		if (/^data\s*:/i.test(trimmed) && trimmed.includes('{')) return true;
		return trimmed.includes('{') && trimmed.includes('}');
	}

	private normalizePayloadSearchQuery(raw: string): string | null {
		const trimmed = raw.trim();
		if (!trimmed) return null;

		const dataLabelMatch = trimmed.match(/^data\s*:\s*([\s\S]+)$/i);
		if (dataLabelMatch) {
			const inner = this.parseRelaxedJsonObject(dataLabelMatch[1]);
			if (inner) return JSON.stringify({ data: inner });
		}

		const obj = this.parseRelaxedJsonObject(trimmed);
		if (!obj || Object.keys(obj).length === 0) return null;
		return JSON.stringify(obj);
	}

	private parseRelaxedJsonObject(raw: string): Record<string, unknown> | null {
		let candidate = raw.trim();
		const start = candidate.indexOf('{');
		const end = candidate.lastIndexOf('}');
		if (start === -1 || end === -1 || end <= start) return null;
		candidate = candidate.slice(start, end + 1);

		const tryParse = (value: string): Record<string, unknown> | null => {
			try {
				const parsed = JSON.parse(value);
				if (parsed !== null && typeof parsed === 'object' && !Array.isArray(parsed)) {
					return parsed as Record<string, unknown>;
				}
			} catch {}
			return null;
		};

		const strict = tryParse(candidate);
		if (strict) return strict;

		const quotedKeys = candidate.replace(/([{,]\s*)([A-Za-z_][\w]*)\s*:/g, '$1"$2":');
		return tryParse(quotedKeys);
	}

	// ------- filters -------

	get hasActiveFilters(): boolean {
		const keys = Object.keys(this.queryParams || {}).filter(
			k => !['sort', 'token', 'next_page_cursor', 'prev_page_cursor', 'direction', 'showLoader', 'startDate', 'endDate'].includes(k)
		);
		return keys.length > 0;
	}

	get dateRangeLabel(): string {
		if (!this.customDateRange || !this.queryParams.startDate || !this.queryParams.endDate) {
			return this.eventLogDefaultDateRangeLabel();
		}
		const format = (value: string) => new Date(value).toLocaleDateString('en-GB', { day: '2-digit', month: 'short' });
		return `${format(this.queryParams.startDate)} - ${format(this.queryParams.endDate)}`;
	}

	refreshEvents(options?: { showLoader?: boolean; showSearchProgress?: boolean }) {
		this.currentPage = 1;
		delete this.queryParams.next_page_cursor;
		delete this.queryParams.prev_page_cursor;
		delete this.queryParams.direction;
		this.getEventLogs({ showLoader: options?.showLoader === true, showSearchProgress: options?.showSearchProgress === true });
	}

	private payloadSearchLeftoverText(raw: string): string {
		const trimmed = raw.trim();
		const dataLabelMatch = trimmed.match(/^data\s*:\s*([\s\S]+)$/i);
		if (dataLabelMatch && this.parseRelaxedJsonObject(dataLabelMatch[1])) {
			return '';
		}
		const start = trimmed.indexOf('{');
		const end = trimmed.lastIndexOf('}');
		if (start === -1 || end === -1 || end <= start) return '';
		return (trimmed.slice(0, start) + trimmed.slice(end + 1)).trim();
	}

	onSearch() {
		if (!this.canSearchEvents) return;
		clearTimeout(this.searchTimeout);
		this.searchTimeout = setTimeout(() => {
			const raw = this.searchString.trim();
			if (!raw) {
				this.payloadAutoScrollKey = '';
				this.queryParams = this.generalService.addFilterToURL({ ...this.queryParams, query: '', body: '' });
				this.refreshEvents({ showSearchProgress: true });
				return;
			}

			const payloadQuery = this.normalizePayloadSearchQuery(raw);
			if (payloadQuery) {
				const leftover = this.payloadSearchLeftoverText(raw);
				this.updatePayloadAutoScrollKey();
				this.queryParams = this.generalService.addFilterToURL({
					...this.queryParams,
					query: leftover || payloadQuery,
					body: leftover ? payloadQuery : ''
				});
				this.refreshEvents({ showSearchProgress: true });
				return;
			}
			if (this.looksLikePayloadSearchInput(raw)) {
				return;
			}

			this.updatePayloadAutoScrollKey();
			this.queryParams = this.generalService.addFilterToURL({ ...this.queryParams, query: raw, body: '' });
			this.refreshEvents({ showSearchProgress: true });
		}, 400);
	}

	getSelectedDateRange(dateRange?: { startDate: string; endDate: string }) {
		if (dateRange) {
			this.customDateRange = true;
			this.queryParams = this.generalService.addFilterToURL({ ...this.queryParams, ...dateRange });
		} else {
			this.customDateRange = false;
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
		this.payloadAutoScrollKey = '';
		this.selectedSourceData = undefined;
		this.datePicker?.clearDate();
		this.customDateRange = false;
		this.queryParams = this.generalService.addFilterToURL({
			...this.queryParams,
			query: '',
			body: '',
			sourceId: '',
			startDate: '',
			endDate: '',
			eventId: '',
			idempotencyKey: ''
		});
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
			if (this.isSearchingEvents) return;
			this.getEventLogs();
		}, 5000);
	}

	checkIfTailModeIsEnabled() {
		const tailModeConfig = localStorage.getItem('EVENT_LOGS_TAIL_MODE');
		return tailModeConfig ? JSON.parse(tailModeConfig) : false;
	}

	// ------- data fetching -------

	async getEventLogs(requestDetails?: { showLoader?: boolean; showSearchProgress?: boolean }) {
		const fetchId = ++this.eventsFetchId;
		if (requestDetails?.showLoader) this.isloadingEvents = true;
		if (requestDetails?.showSearchProgress) this.isSearchingEvents = true;
		this.fetchError = false;
		this.fetchErrorMessage = '';
		this.searchTimedOut = false;

		try {
			const cleanedQuery: any = JSON.parse(JSON.stringify(this.queryParams));
			delete cleanedQuery.showLoader;
			if (cleanedQuery.query) cleanedQuery.query = this.queryParamString(cleanedQuery.query);
			if (cleanedQuery.body) cleanedQuery.body = this.queryParamString(cleanedQuery.body);
			const eventsResponse = await this.eventsService.getEvents(this.eventsListQueryParams(cleanedQuery));
			if (fetchId !== this.eventsFetchId) return eventsResponse;
			this.events = eventsResponse.data;

			this.displayedEvents = await this.generalService.setContentDisplayed(eventsResponse.data.content, this.queryParams?.sort || 'desc');

			const selectedStillVisible = !!this.events?.content?.some(event => event.uid === this.eventsDetailsItem?.uid);
			if (!this.eventsDetailsItem || !selectedStillVisible) {
				this.eventsDetailsItem = this.events?.content?.[0];
			}

			if (this.eventsDetailsItem?.uid) {
				if (this.sidebarLoadedForEventId !== this.eventsDetailsItem.uid || this.sidebarLoadedRange !== this.sidebarDateWindowKey()) {
					this.getEventDeliveriesForSidebar(this.eventsDetailsItem.uid);
					this.getDuplicateEvents(this.eventsDetailsItem);
				}
				this.updatePayloadAutoScrollKey(this.eventsDetailsItem.uid);
			} else {
				this.isLoadingSidebarDeliveries = false;
				this.sidebarEventDeliveries = [];
				this.sidebarLoadedForEventId = '';
				this.sidebarLoadedRange = '';
			}

			this.isloadingEvents = false;
			this.isSearchingEvents = false;

			return eventsResponse;
		} catch (error: unknown) {
			if (fetchId !== this.eventsFetchId) return error;
			this.isloadingEvents = false;
			this.isSearchingEvents = false;
			this.fetchError = true;
			const classified = classifyEventLogsFetchError(error);
			this.searchTimedOut = classified.searchTimedOut;
			this.fetchErrorMessage = classified.fetchErrorMessage;
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
		if (!eventId) {
			this.isLoadingSidebarDeliveries = false;
			this.sidebarEventDeliveries = [];
			this.sidebarLoadedForEventId = '';
			this.sidebarLoadedRange = '';
			return;
		}

		const fetchId = ++this.sidebarFetchId;
		const range = this.eventsListQueryParams();
		const rangeKey = this.sidebarDateWindowKey(range);
		this.isLoadingSidebarDeliveries = true;
		this.sidebarEventDeliveries = [];
		this.sidebarLoadedForEventId = '';
		this.sidebarLoadedRange = '';

		try {
			const response = await this.eventsService.getEventDeliveries({
				eventId,
				startDate: range.startDate,
				endDate: range.endDate
			});
			if (fetchId !== this.sidebarFetchId) return;
			this.sidebarEventDeliveries = response.data?.content || [];
			this.isLoadingSidebarDeliveries = false;
			this.sidebarLoadedForEventId = eventId;
			this.sidebarLoadedRange = rangeKey;
			return;
		} catch (error) {
			if (fetchId !== this.sidebarFetchId) return;
			this.sidebarEventDeliveries = [];
			this.isLoadingSidebarDeliveries = false;
			return error;
		}
	}

	private sidebarDateWindowKey(range: FILTER_QUERY_PARAM = this.eventsListQueryParams()): string {
		return `${range.startDate || ''}\0${range.endDate || ''}`;
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
		const total = this.hasActiveSearch ? undefined : this.events?.pagination?.total;

		return total ? `${start}-${end} of ${total}` : `${start}-${end}`;
	}

	groupDateLabel(date: string): string {
		return date.replace(',', '');
	}

	// ------- retries -------

	async fetchRetryCount() {
		this.fetchingCount = true;
		try {
			const response = await this.eventsLogService.getRetryCount(this.eventsListQueryParams());

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
			const response = await this.eventsLogService.batchRetryEvent(this.eventsListQueryParams());

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
		return this.hasMatchedSubscription(event) ? 'Matched' : 'Unmatched';
	}

	routeChipClass(event?: EVENT): string {
		if (this.hasMatchedSubscription(event)) return 'bg-success-a3 text-success-11';
		return 'bg-new.surface-muted text-new.text-secondary';
	}

	// Prefer the server's reason. Without it we can only guess, and the guess is
	// wrong for events that failed before any endpoint was resolved, such as a
	// dynamic URL that matched no endpoint URL template.
	noDeliveriesReason(event?: EVENT): string {
		if (event?.failure_reason) return event.failure_reason;
		if (this.hasMatchedSubscription(event)) return 'No event delivery attempt for this event yet.';
		return 'No matching subscription. This event was accepted but did not match any subscription event type or filter.';
	}

	private get projectConfig() {
		return this.projectService.activeProjectDetails || this.privateService.getProjectDetails;
	}

	private formatApiDate(date: Date): string {
		return date.toISOString().slice(0, -5);
	}

	/** Default list window matches the API when no custom date range is selected. */
	eventLogDefaultDateRange(): { startDate: string; endDate: string } {
		const now = new Date();
		const end = new Date(now.getFullYear(), now.getMonth(), now.getDate(), 23, 59, 59);
		const start = new Date(now);
		start.setDate(start.getDate() - 7);
		start.setHours(0, 0, 0, 0);

		return {
			startDate: this.formatApiDate(start),
			endDate: this.formatApiDate(end)
		};
	}

	eventLogDefaultDateRangeLabel(): string {
		return 'Last 7 days';
	}

	private eventsListQueryParams(base: FILTER_QUERY_PARAM = this.queryParams): FILTER_QUERY_PARAM {
		if (this.customDateRange && base.startDate && base.endDate) {
			return base;
		}
		return { ...base, ...this.eventLogDefaultDateRange() };
	}
}
