import { Component, ElementRef, EventEmitter, Input, OnInit, Output, SimpleChanges, ViewChild } from '@angular/core';
import { ActivatedRoute, Router } from '@angular/router';
import { format } from 'date-fns';
import { fromEvent, Observable } from 'rxjs';
import { debounceTime, distinctUntilChanged, map, startWith, switchMap } from 'rxjs/operators';
import { APP } from 'src/app/models/app.model';
import { EVENT_DELIVERY, EVENT_DELIVERY_ATTEMPT } from 'src/app/models/event.model';
import { PAGINATION } from 'src/app/models/global.model';
import { HTTP_RESPONSE } from 'src/app/models/http.model';
import { TimeFilterComponent } from 'src/app/private/components/time-filter/time-filter.component';
import { GeneralService } from 'src/app/services/general/general.service';
import { DropdownComponent } from 'src/app/components/dropdown/dropdown.component';
import { EventsService } from '../events.service';
import { PrivateService } from 'src/app/private/private.service';
import { SOURCE } from 'src/app/models/group.model';
import { DatePickerComponent } from 'src/app/components/date-picker/date-picker.component';

@Component({
	selector: 'app-event-deliveries',
	templateUrl: './event-deliveries.component.html',
	styleUrls: ['./event-deliveries.component.scss']
})
export class EventDeliveriesComponent implements OnInit {
	@Output() pushEventDeliveries = new EventEmitter<any>();
	@Input() eventDeliveryFilteredByEventId?: string;
	dateOptions = ['Last Year', 'Last Month', 'Last Week', 'Yesterday'];
	eventDeliveryStatuses = ['Success', 'Failure', 'Retry', 'Scheduled', 'Processing', 'Discarded'];
	eventDelTableHead: string[] = ['Status', 'Event Type', 'Attempts', 'Max Attempts', 'Time Created', '', ''];
	eventDeliveriesStatusFilterActive = false;
	fetchingCount = false;
	showBatchRetryModal = false;
	isloadingEventDeliveries = false;
	isloadingDeliveryAttempts = false;
	isRetrying = false;
	dateFiltersFromURL: { startDate: string | Date; endDate: string | Date } = { startDate: '', endDate: '' };
	batchRetryCount!: number;
	eventDeliveriesApp?: string;
	eventDeliveriesSource?: string;
	eventDeliveryIndex!: number;
	eventDeliveriesPage: number = 1;
	selectedEventsFromEventDeliveriesTable: string[] = [];
	displayedEventDeliveries!: { date: string; content: EVENT_DELIVERY[] }[];
	eventDeliveries?: { pagination: PAGINATION; content: EVENT_DELIVERY[] };
	sidebarEventDeliveries!: EVENT_DELIVERY[];
	eventDeliveryAtempt!: EVENT_DELIVERY_ATTEMPT;
	eventDeliveryFilteredByStatus: string[] = [];
	eventDelsTimeFilterData: { startTime: string; endTime: string } = { startTime: 'T00:00:00', endTime: 'T23:59:59' };
	eventsDelAppsFilter$!: Observable<APP[]>;
	@ViewChild('eventDelsAppsFilter', { static: true }) eventDelsAppsFilter!: ElementRef;
	@ViewChild('datePicker', { static: true }) datePicker!: DatePickerComponent;
	@ViewChild('eventDeliveryTimerFilter', { static: true }) eventDeliveryTimerFilter!: TimeFilterComponent;
	@ViewChild('appsFilterDropdown', { static: true }) appsFilterDropdown!: DropdownComponent;
	@ViewChild('sourcesFilterDropdown', { static: true }) sourcesFilterDropdown!: DropdownComponent;
	appPortalToken = this.route.snapshot.params?.token;
	filterSources: SOURCE[] = [];

	constructor(private generalService: GeneralService, private eventsService: EventsService, private route: ActivatedRoute, private router: Router, public privateService: PrivateService) {}

	ngAfterViewInit() {
		if (!this.appPortalToken) {
			this.eventsDelAppsFilter$ = fromEvent<any>(this.eventDelsAppsFilter?.nativeElement, 'keyup').pipe(
				map(event => event.target.value),
				startWith(''),
				debounceTime(500),
				distinctUntilChanged(),
				switchMap(search => this.getAppsForFilter(search))
			);
		}
	}

	ngOnInit() {
		this.getFiltersFromURL();
		this.getEventDeliveries();
		this.getSourcesForFilter();
	}

	ngOnChanges(changes: SimpleChanges) {
		const prevValue = changes?.eventDeliveryFilteredByEventId.previousValue;
		const currentValue = changes?.eventDeliveryFilteredByEventId.currentValue;
		if (currentValue !== prevValue) this.getEventDeliveries();
	}

	getFiltersFromURL() {
		const filters = this.route.snapshot.queryParams;
		if (Object.keys(filters).length == 0) return;

		this.dateFiltersFromURL = {
			startDate: filters.eventDelsStartDate ? new Date(filters.eventDelsStartDate) : '',
			endDate: filters.eventDelsEndDate ? new Date(filters.eventDelsEndDate) : ''
		};
		this.eventDeliveriesApp = filters.eventDelsApp ?? '';
		this.eventDeliveryFilteredByStatus = filters.eventDelsStatus ? JSON.parse(filters.eventDelsStatus) : [];
	}

	async getEventDeliveries(requestDetails?: { page?: number; addToURL?: boolean; fromFilter?: boolean }): Promise<HTTP_RESPONSE> {
		const page = requestDetails?.page || this.route.snapshot.queryParams.page || 1;
		if (requestDetails?.addToURL) this.addFilterToURL();
		const { startDate, endDate } = this.setDateForFilter({ ...this.dateFiltersFromURL, ...this.eventDelsTimeFilterData });
		this.isloadingEventDeliveries = true;

		try {
			const eventDeliveriesResponse = await this.eventDeliveriesRequest({ pageNo: page, eventId: this.eventDeliveryFilteredByEventId, startDate, endDate });
			this.eventDeliveries = eventDeliveriesResponse.data;
			this.displayedEventDeliveries = this.generalService.setContentDisplayed(eventDeliveriesResponse.data.content);

			this.pushEventDeliveries.emit(this.eventDeliveries);

			this.isloadingEventDeliveries = false;
			return eventDeliveriesResponse;
		} catch (error: any) {
			this.isloadingEventDeliveries = false;
			return error;
		}
	}

	async eventDeliveriesRequest(requestDetails: { pageNo?: number; eventId?: string; startDate?: string; endDate?: string }): Promise<HTTP_RESPONSE> {
		let eventDeliveryStatusFilterQuery = '';
		this.eventDeliveryFilteredByStatus.length > 0 ? (this.eventDeliveriesStatusFilterActive = true) : (this.eventDeliveriesStatusFilterActive = false);
		this.eventDeliveryFilteredByStatus.forEach((status: string) => (eventDeliveryStatusFilterQuery += `&status=${status}`));

		try {
			const eventDeliveriesResponse = await this.eventsService.getEventDeliveries({
				eventId: requestDetails.eventId || '',
				pageNo: requestDetails.pageNo || 1,
				startDate: requestDetails.startDate,
				endDate: requestDetails.endDate,
				appId: this.eventDeliveriesApp || '',
				statusQuery: eventDeliveryStatusFilterQuery || '',
				token: this.appPortalToken,
				sourceId: this.eventDeliveriesSource || ''
			});
			return eventDeliveriesResponse;
		} catch (error: any) {
			return error;
		}
	}

	addFilterToURL() {
		const currentURLfilters = this.route.snapshot.queryParams;
		const queryParams: any = {};
		const { startDate, endDate } = this.setDateForFilter({ ...this.dateFiltersFromURL, ...this.eventDelsTimeFilterData });
		if (startDate) queryParams.eventDelsStartDate = startDate;
		if (endDate) queryParams.eventDelsEndDate = endDate;
		if (this.eventDeliveriesApp) queryParams.eventDelsApp = this.eventDeliveriesApp;
		queryParams.eventDelsSource = this.eventDeliveriesSource;
		queryParams.eventDelsStatus = this.eventDeliveryFilteredByStatus.length > 0 ? JSON.stringify(this.eventDeliveryFilteredByStatus) : '';

		this.router.navigate([], { queryParams: Object.assign({}, currentURLfilters, queryParams) });
	}

	setDateForFilter(requestDetails: { startDate: any; endDate: any; startTime?: string; endTime?: string }) {
		if (!requestDetails.endDate && !requestDetails.startDate) return { startDate: '', endDate: '' };
		const startDate = requestDetails.startDate ? `${format(requestDetails.startDate, 'yyyy-MM-dd')}${requestDetails?.startTime || 'T00:00:00'}` : '';
		const endDate = requestDetails.endDate ? `${format(requestDetails.endDate, 'yyyy-MM-dd')}${requestDetails?.endTime || 'T23:59:59'}` : '';
		return { startDate, endDate };
	}

	checkIfEventDeliveryStatusFilterOptionIsSelected(status: string): boolean {
		return this.eventDeliveryFilteredByStatus?.length > 0 ? this.eventDeliveryFilteredByStatus.includes(status) : false;
	}

	updateEventDevliveryStatusFilter(status: string, isChecked: any) {
		if (isChecked.target.checked) {
			this.eventDeliveryFilteredByStatus.push(status);
		} else {
			let index = this.eventDeliveryFilteredByStatus.findIndex(x => x === status);
			this.eventDeliveryFilteredByStatus.splice(index, 1);
		}
	}

	getSelectedDateRange(dateRange: { startDate: Date; endDate: Date }) {
		this.dateFiltersFromURL = { ...dateRange };
		this.getEventDeliveries({ addToURL: true });
	}

	clearFilters(filterType?: 'app' | 'time' | 'date' | 'status' | 'source') {
		const activeFilters = Object.assign({}, this.route.snapshot.queryParams);
		let filterItems: string[] = [];
		this.datePicker.clearDate();
		this.eventDeliveryTimerFilter.filterStartHour = 0;
		this.eventDeliveryTimerFilter.filterStartMinute = 0;
		this.eventDeliveryTimerFilter.filterEndHour = 23;
		this.eventDeliveryTimerFilter.filterEndMinute = 59;

		switch (filterType) {
			case 'app':
				filterItems = ['eventDelsApp'];
				this.eventDeliveriesApp = undefined;
				this.appsFilterDropdown.show = false;
				break;
			case 'date':
				filterItems = ['eventDelsStartDate', 'eventDelsEndDate'];
				this.dateFiltersFromURL = { startDate: '', endDate: '' };
				break;
			case 'status':
				filterItems = ['eventDelsStatus'];
				this.eventDeliveryFilteredByStatus = [];
				break;
			case 'time':
				filterItems = ['eventDelsTime'];
				this.eventDelsTimeFilterData = { startTime: 'T00:00:00', endTime: 'T23:59:59' };
				break;
			case 'source':
				this.sourcesFilterDropdown.show = false;
				filterItems = ['eventDelsSource'];
				this.eventDeliveriesSource = undefined;
				break;
			default:
				filterItems = ['eventDelsStartDate', 'eventDelsTime', 'eventDelsEndDate', 'eventDelsApp', 'eventDelsStatus', 'eventDelsSource'];
				this.eventDeliveriesApp = undefined;
				this.eventDeliveriesSource = undefined;
				this.dateFiltersFromURL = { startDate: '', endDate: '' };
				this.eventDeliveryFilteredByEventId = undefined;
				this.eventDeliveryFilteredByStatus = [];
				this.eventDelsTimeFilterData = { startTime: 'T00:00:00', endTime: 'T23:59:59' };
				break;
		}

		this.eventDeliveryFilteredByEventId = undefined;

		filterItems.forEach(key => (activeFilters.hasOwnProperty(key) ? delete activeFilters[key] : null));
		this.router.navigate(['./'], { relativeTo: this.route, queryParams: activeFilters });
		this.getEventDeliveries();
	}

	async fetchRetryCount() {
		let eventDeliveryStatusFilterQuery = '';
		this.eventDeliveryFilteredByStatus.length > 0 ? (this.eventDeliveriesStatusFilterActive = true) : (this.eventDeliveriesStatusFilterActive = false);
		this.eventDeliveryFilteredByStatus.forEach((status: string) => (eventDeliveryStatusFilterQuery += `&status=${status}`));
		const { startDate, endDate } = this.setDateForFilter(this.dateFiltersFromURL);
		this.fetchingCount = true;
		try {
			const response = await this.eventsService.getRetryCount({
				eventId: this.eventDeliveryFilteredByEventId || '',
				pageNo: this.eventDeliveriesPage || 1,
				startDate: startDate,
				endDate: endDate,
				appId: this.eventDeliveriesApp || '',
				statusQuery: eventDeliveryStatusFilterQuery || '',
				token: this.appPortalToken
			});

			this.batchRetryCount = response.data.num;
			this.fetchingCount = false;
			this.showBatchRetryModal = true;
		} catch (error) {
			this.fetchingCount = false;
		}
	}

	async getAppsForFilter(search: string): Promise<APP[]> {
		return await (
			await this.eventsService.getApps({ pageNo: 1, searchString: search })
		).data.content;
	}

	async getSourcesForFilter() {
		try {
			const sourcesResponse = (await this.privateService.getSources()).data.content;
			this.filterSources = sourcesResponse;
		} catch (error) {}
	}

	updateAppFilter(appId: string, isChecked: any) {
		isChecked.target.checked ? (this.eventDeliveriesApp = appId) : (this.eventDeliveriesApp = undefined);
		this.getEventDeliveries({ addToURL: true, fromFilter: true });
	}

	updateSourceFilter(sourceId: string, isChecked: any) {
		isChecked.target.checked ? (this.eventDeliveriesSource = sourceId) : (this.eventDeliveriesSource = undefined);
		this.getEventDeliveries({ addToURL: true, fromFilter: true });
	}

	async retryEvent(requestDetails: { e: any; index: number; eventDeliveryId: string }) {
		requestDetails.e.stopPropagation();
		try {
			const response = await this.eventsService.retryEvent({ eventId: requestDetails.eventDeliveryId, token: this.appPortalToken });
			this.generalService.showNotification({ message: response.message, style: 'success' });
			this.getEventDeliveries();
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
			const response = await this.eventsService.forceRetryEvent({ body: payload, token: this.appPortalToken });
			this.generalService.showNotification({ message: response.message, style: 'success' });
			this.getEventDeliveries();
		} catch (error) {
			return error;
		}
	}

	async batchRetryEvent() {
		let eventDeliveryStatusFilterQuery = '';
		this.eventDeliveryFilteredByStatus.length > 0 ? (this.eventDeliveriesStatusFilterActive = true) : (this.eventDeliveriesStatusFilterActive = false);
		this.eventDeliveryFilteredByStatus.forEach((status: string) => (eventDeliveryStatusFilterQuery += `&status=${status}`));
		const { startDate, endDate } = this.setDateForFilter(this.dateFiltersFromURL);
		this.isRetrying = true;

		try {
			const response = await this.eventsService.batchRetryEvent({
				eventId: this.eventDeliveryFilteredByEventId || '',
				pageNo: this.eventDeliveriesPage || 1,
				startDate: startDate,
				endDate: endDate,
				appId: this.eventDeliveriesApp || '',
				statusQuery: eventDeliveryStatusFilterQuery || '',
				token: this.appPortalToken
			});

			this.generalService.showNotification({ message: response.message, style: 'success' });
			this.getEventDeliveries();
			this.showBatchRetryModal = false;
			this.isRetrying = false;
		} catch (error) {
			this.isRetrying = false;
			return error;
		}
	}
}
