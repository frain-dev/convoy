import { AfterViewInit, Component, ElementRef, EventEmitter, OnInit, Output, ViewChild } from '@angular/core';
import { ActivatedRoute } from '@angular/router';
import { fromEvent, Observable } from 'rxjs';
import { debounceTime, distinctUntilChanged, map, startWith } from 'rxjs/operators';
import { ENDPOINT } from 'src/app/models/endpoint.model';
import { EVENT_DELIVERY, FILTER_QUERY_PARAM } from 'src/app/models/event.model';
import { CURSOR, PAGINATION } from 'src/app/models/global.model';
import { HTTP_RESPONSE } from 'src/app/models/global.model';
import { GeneralService } from 'src/app/services/general/general.service';
import { EventsService } from '../events.service';
import { PrivateService } from 'src/app/private/private.service';
import { SOURCE } from 'src/app/models/source.model';
import { DatePickerComponent } from 'src/app/components/date-picker/date-picker.component';
import { ProjectService } from '../../project.service';
import { Location } from '@angular/common';

@Component({
	selector: 'app-event-deliveries',
	templateUrl: './event-deliveries.component.html',
	styleUrls: ['./event-deliveries.component.scss']
})
export class EventDeliveriesComponent implements OnInit {
	@Output() pushEventDeliveries = new EventEmitter<any>();
	eventDeliveryStatuses = ['Success', 'Failure', 'Retry', 'Scheduled', 'Processing', 'Discarded'];
	eventDelTableHead: string[] = ['Status', 'Event type', this.projectService.activeProjectDetails?.type == 'incoming' ? 'Subscription' : 'Endpoint', 'Attempts', 'Next Attempt', 'Time', '', ''];
	fetchingCount = false;
	showBatchRetryModal = false;
	isloadingEventDeliveries = false;
	isRetrying = false;
	batchRetryCount!: number;
	eventDeliveriesEndpoint?: string;
	eventDeliveriesEndpointData?: ENDPOINT;
	eventDeliveriesSource?: string;
	eventDeliveriesSourceData?: SOURCE;
	displayedEventDeliveries!: { date: string; content: any[] }[];
	eventDeliveries?: { pagination: PAGINATION; content: EVENT_DELIVERY[] };
	eventDeliveryFilteredByStatus: string[] = [];
	eventsDelEndpointFilter$!: Observable<ENDPOINT[]>;
	@ViewChild('eventTypeForm', { static: false }) eventDelsEventsTypeForm!: ElementRef;
	@ViewChild('eventDelsEndpointFilter', { static: true }) eventDelsEndpointFilter!: ElementRef;
	@ViewChild('datePicker', { static: true }) datePicker!: DatePickerComponent;
	@ViewChild('batchRetryDialog', { static: true }) dialog!: ElementRef<HTMLDialogElement>;
	portalToken = this.route.snapshot.queryParams?.token;
	filterSources: SOURCE[] = [];
	queryParams?: FILTER_QUERY_PARAM;
	getEventDeliveriesInterval: any;
	enableTailMode = false;
	loadingFilterEndpoints = false;
	eventDelEventType?: string;
	eventsTypeSearchString!: string;
	filterOptions = [
		{ name: 'Date', show: false },
		{ name: 'Status', show: false },
		{ name: 'Source', show: false },
		{ name: 'Endpoint', show: false },
		{ name: 'Event type', show: false }
	];
	sortOrder: 'asc' | 'desc' = 'desc';

	constructor(private generalService: GeneralService, private eventsService: EventsService, public route: ActivatedRoute, public projectService: ProjectService, public privateService: PrivateService, private _location: Location) {}

	async ngOnInit() {
		const data = this.getFiltersFromURL();
		this.getEventDeliveries({ ...data, showLoader: true });
		if (this.checkIfTailModeIsEnabled()) this.getEventDeliveriesAtInterval();

		this.projectService.activeProjectDetails?.type == 'incoming' ? this.filterOptions.splice(3, 2) : this.filterOptions.splice(2, 1);

		if (this.eventDeliveriesSource) this.eventDeliveriesSourceData = await this.getSelectedSourceData();

		if (!this.portalToken || this.projectService.activeProjectDetails?.type == 'incoming') this.getSourcesForFilter();
	}

	ngOnDestroy() {
		clearInterval(this.getEventDeliveriesInterval);
	}

	toggleSortOrder() {
		this.sortOrder === 'asc' ? (this.sortOrder = 'desc') : (this.sortOrder = 'asc');
		const data = this.addFilterToURL({ sort: this.sortOrder });
		this.checkIfTailModeIsEnabled() ? this.toggleTailMode(false, 'on') : this.toggleTailMode(false, 'off');
		this.getEventDeliveries({ ...data, showLoader: true });
	}

	setEventType() {
		console.log(this.eventsTypeSearchString);
		this.eventDelEventType = this.eventsTypeSearchString;
		const data = this.addFilterToURL({ eventType: this.eventsTypeSearchString });
		this.checkIfTailModeIsEnabled() ? this.toggleTailMode(false, 'on') : this.toggleTailMode(false, 'off');
		this.getEventDeliveries({ ...data, showLoader: true });
		this.toggleFilter('Event type', false);
	}

	fetchEventDeliveries(requestDetails?: FILTER_QUERY_PARAM) {
		const data = requestDetails;
	}

	getFiltersFromURL() {
		this.queryParams = { ...this.queryParams, ...this.route.snapshot.queryParams };

		// set filter status if any exists in URL
		this.eventDeliveryFilteredByStatus = this.queryParams.status ? JSON.parse(this.queryParams.status) : [];

		this.eventDeliveriesSource = this.queryParams?.sourceId;
		this.eventDeliveriesEndpoint = this.queryParams?.endpointId;

		this.eventDelEventType = this.queryParams?.eventType;

		return this.queryParams;
	}

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
			const data = { ...this.queryParams, ...this.route.snapshot.queryParams };
			this.getEventDeliveries(data);
		}, 5000);
	}

	async getEventDeliveries(requestDetails?: FILTER_QUERY_PARAM): Promise<HTTP_RESPONSE> {
		if (requestDetails?.showLoader) this.isloadingEventDeliveries = true;

		try {
			const eventDeliveriesResponse = await this.eventDeliveriesRequest(requestDetails);
			this.eventDeliveries = eventDeliveriesResponse.data;

			this.displayedEventDeliveries = this.setEventDeliveriesContent(eventDeliveriesResponse.data.content);

			this.isloadingEventDeliveries = false;
			return eventDeliveriesResponse;
		} catch (error: any) {
			this.isloadingEventDeliveries = false;
			return error;
		}
	}

	setEventDeliveriesContent(eventDeliveriesData: any[]) {
		const eventIds: any = [];
		const finalEventDels: any = [];
		let filteredEventDeliveries: any = [];

		const filteredEventDeliveriesByDate = this.generalService.setContentDisplayed(eventDeliveriesData);

		eventDeliveriesData.forEach((item: any) => {
			eventIds.push(item.event_id);
		});
		const uniqueEventIds = [...new Set(eventIds)];

		filteredEventDeliveriesByDate.forEach((eventDelivery: any) => {
			uniqueEventIds.forEach(eventId => {
				const filteredDeliveriesByEventId = eventDelivery.content.filter((item: any) => item.event_id === eventId);
				filteredEventDeliveries.push({ date: eventDelivery.date, event_id: eventId, eventDeliveries: filteredDeliveriesByEventId });
			});

			filteredEventDeliveries = filteredEventDeliveries.filter((item: any) => item.eventDeliveries.length !== 0);
			const uniqueEventDels = filteredEventDeliveries.filter((eventDels: any) => eventDelivery.date === eventDels.date);
			finalEventDels.push({ date: eventDelivery.date, content: uniqueEventDels });
		});

		return finalEventDels;
	}

	async eventDeliveriesRequest(requestDetails?: FILTER_QUERY_PARAM): Promise<HTTP_RESPONSE> {
		try {
			const eventDeliveriesResponse = await this.eventsService.getEventDeliveries(requestDetails);
			return eventDeliveriesResponse;
		} catch (error: any) {
			return error;
		}
	}

	addFilterToURL(params?: FILTER_QUERY_PARAM) {
		this.queryParams = { ...this.queryParams, ...this.route.snapshot.queryParams, ...params };

		if (!params?.next_page_cursor) delete this.queryParams.next_page_cursor;
		if (!params?.prev_page_cursor) delete this.queryParams.prev_page_cursor;

		const cleanedQuery: any = Object.fromEntries(Object.entries(this.queryParams).filter(([_, q]) => q !== '' && q !== undefined && q !== null));
		const queryParams = new URLSearchParams(cleanedQuery).toString();
		this._location.go(`${location.pathname}?${queryParams}`);

		return this.queryParams;
	}

	checkIfEventDeliveryStatusFilterOptionIsSelected(status: string): boolean {
		return this.eventDeliveryFilteredByStatus?.length > 0 ? this.eventDeliveryFilteredByStatus.includes(status) : false;
	}

	isAnyFilterSelected(): Boolean {
		return (this.queryParams && Object.keys(this.queryParams).length > 0) || false;
	}

	toggleFilter(filterValue: string, show: boolean) {
		this.filterOptions.forEach(filter => {
			if (filter.name === filterValue) filter.show = show;
		});
	}

	showFilter(filterValue: string): boolean {
		return this.filterOptions.find(filter => filter.name === filterValue)?.show || false;
	}

	selectStatusFilter(status: string) {
		if (!this.eventDeliveryFilteredByStatus?.includes(status)) this.eventDeliveryFilteredByStatus.push(status);
		this.toggleFilter('Status', false);
		this.getSelectedStatusFilter();
	}

	removeStatusFilter(status: string) {
		this.eventDeliveryFilteredByStatus = this.eventDeliveryFilteredByStatus.filter(e => e !== status);
		this.getSelectedStatusFilter();
	}

	updateEventDevliveryStatusFilter(status: string, isChecked: any) {
		if (isChecked.target.checked) {
			this.eventDeliveryFilteredByStatus.push(status);
		} else {
			let index = this.eventDeliveryFilteredByStatus.findIndex(x => x === status);
			this.eventDeliveryFilteredByStatus.splice(index, 1);
		}
	}

	getSelectedDateRange(dateRange: { startDate: string; endDate: string }) {
		const data = this.addFilterToURL(dateRange);
		this.checkIfTailModeIsEnabled() ? this.toggleTailMode(false, 'on') : this.toggleTailMode(false, 'off');
		this.getEventDeliveries({ ...data, showLoader: true });
	}

	getSelectedStatusFilter() {
		const eventDelsStatus = this.eventDeliveryFilteredByStatus.length > 0 ? JSON.stringify(this.eventDeliveryFilteredByStatus) : '';
		const data = this.addFilterToURL({ status: eventDelsStatus });
		this.checkIfTailModeIsEnabled() ? this.toggleTailMode(false, 'on') : this.toggleTailMode(false, 'off');
		this.getEventDeliveries({ ...data, showLoader: true });
	}

	clearFilters(filterType?: 'startDate' | 'endDate' | 'eventId' | 'endpointId' | 'status' | 'sourceId' | 'next_page_cursor' | 'prev_page_cursor' | 'direction' | 'eventType') {
		if (filterType && this.queryParams) {
			// if filter to clear start date or end date, it means clear date filter. :)
			if (filterType === 'startDate' || filterType === 'endDate') {
				delete this.queryParams['startDate'];
				delete this.queryParams['endDate'];
			} else if (filterType === 'eventId') {
				delete this.queryParams['eventId'];
				delete this.queryParams['idempotencyKey'];
			} else if (filterType === 'endpointId') {
				this.eventDeliveriesEndpoint = '';
				delete this.queryParams['endpointId'];
			} else if (filterType === 'sourceId') {
				this.eventDeliveriesSource = '';
				delete this.queryParams['sourceId'];
			} else if (filterType === 'eventType') {
				this.eventDelEventType = '';
				this.eventsTypeSearchString = '';
				delete this.queryParams['eventType'];
			} else delete this.queryParams[filterType];

			const cleanedQuery: any = Object.fromEntries(Object.entries(this.queryParams).filter(([_, q]) => q !== '' && q !== undefined && q !== null));
			const queryParams = new URLSearchParams(cleanedQuery).toString();
			this._location.go(`${location.pathname}?${queryParams}`);
		} else {
			this.datePicker.clearDate();
			this.queryParams = {};
			this._location.go(`${location.pathname}`);
		}

		this.checkIfTailModeIsEnabled() ? this.toggleTailMode(false, 'on') : this.toggleTailMode(false, 'off');
		this.getEventDeliveries({ showLoader: true });
	}

	async fetchRetryCount() {
		if (!this.queryParams) return;

		this.fetchingCount = true;
		try {
			const response = await this.eventsService.getRetryCount(this.queryParams);

			this.batchRetryCount = response.data.num;
			this.fetchingCount = false;
			this.dialog.nativeElement.showModal();
		} catch (error) {
			this.fetchingCount = false;
		}
	}

	async getSelectedSourceData(): Promise<SOURCE> {
		return await (await this.privateService.getSources()).data.content.find((item: SOURCE) => item.uid === this.eventDeliveriesSource);
	}

	async getSourcesForFilter() {
		try {
			const sourcesResponse = (await this.privateService.getSources()).data.content;
			this.filterSources = sourcesResponse;
		} catch (error) {}
	}

	updateEndpointFilter() {
		const data = this.addFilterToURL({ endpointId: this.eventDeliveriesEndpoint });
		this.checkIfTailModeIsEnabled() ? this.toggleTailMode(false, 'on') : this.toggleTailMode(false, 'off');
		this.getEventDeliveries({ ...data, showLoader: true });
	}

	updateSourceFilter() {
		const data = this.addFilterToURL({ sourceId: this.eventDeliveriesSource });
		this.checkIfTailModeIsEnabled() ? this.toggleTailMode(false, 'on') : this.toggleTailMode(false, 'off');
		this.getEventDeliveries({ ...data, showLoader: true });
	}

	paginateEvents(event: CURSOR) {
		const data = this.addFilterToURL({ next_page_cursor: event.next_page_cursor, prev_page_cursor: event.prev_page_cursor });
		if (this.checkIfTailModeIsEnabled()) this.toggleTailMode(false, 'off');
		this.getEventDeliveries({ ...data, showLoader: true });
	}

	async retryEvent(requestDetails: { e: any; index: number; eventDeliveryId: string }) {
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
	async forceRetryEvent(requestDetails: { e: any; index: number; eventDeliveryId: string }) {
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

	async batchRetryEvent() {
		if (!this.queryParams) return;
		this.isRetrying = true;

		try {
			const response = await this.eventsService.batchRetryEvent(this.queryParams);

			this.generalService.showNotification({ message: response.message, style: 'success' });
			this.dialog.nativeElement.close();
			this.isRetrying = false;
			return;
		} catch (error) {
			this.isRetrying = false;
			return error;
		}
	}

	toggleSouceFilter(event: any, sourceId: string) {
		this.eventDeliveriesSource = event.target.checked ? sourceId : '';
	}
}
