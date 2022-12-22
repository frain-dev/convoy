import { Component, ElementRef, OnInit, ViewChild } from '@angular/core';
import { CommonModule } from '@angular/common';
import { PrivateService } from 'src/app/private/private.service';
import { ActivatedRoute, Router, RouterModule } from '@angular/router';
import { CardComponent } from 'src/app/components/card/card.component';
import { ButtonComponent } from 'src/app/components/button/button.component';
import { PAGINATION } from 'src/app/models/global.model';
import { EmptyStateComponent } from 'src/app/components/empty-state/empty-state.component';
import { TableLoaderModule } from 'src/app/private/components/table-loader/table-loader.module';
import { TagComponent } from 'src/app/components/tag/tag.component';
import { TableComponent, TableCellComponent, TableRowComponent, TableHeadCellComponent, TableHeadComponent } from 'src/app/components/table/table.component';
import { EventLogsService } from './event-logs.service';
import { GeneralService } from 'src/app/services/general/general.service';
import { ENDPOINT } from 'src/app/models/endpoint.model';
import { HTTP_RESPONSE } from 'src/app/models/http.model';
import { format } from 'date-fns';
import { SOURCE } from 'src/app/models/group.model';
import { EVENT, EVENT_DELIVERY } from 'src/app/models/event.model';
import { TimePickerComponent } from 'src/app/components/time-picker/time-picker.component';
import { DatePickerComponent } from 'src/app/components/date-picker/date-picker.component';
import { fromEvent, Observable } from 'rxjs';
import { debounceTime, distinctUntilChanged, map, startWith, switchMap } from 'rxjs/operators';
import { StatusColorModule } from 'src/app/pipes/status-color/status-color.module';
import { PrismModule } from 'src/app/private/components/prism/prism.module';
import { LoaderModule } from 'src/app/private/components/loader/loader.module';
import { FormsModule } from '@angular/forms';
import { DropdownComponent } from 'src/app/components/dropdown/dropdown.component';
import { ModalComponent } from 'src/app/components/modal/modal.component';

@Component({
	selector: 'convoy-event-logs',
	standalone: true,
	imports: [
		CommonModule,
		RouterModule,
		FormsModule,
		StatusColorModule,
		PrismModule,
		LoaderModule,
		CardComponent,
		ButtonComponent,
		EmptyStateComponent,
		TagComponent,
		TableLoaderModule,
		TableComponent,
		TableHeadComponent,
		TableRowComponent,
		TableHeadCellComponent,
		TableCellComponent,
		TimePickerComponent,
		DatePickerComponent,
		DropdownComponent,
		ModalComponent
	],
	templateUrl: './event-logs.component.html',
	styleUrls: ['./event-logs.component.scss']
})
export class EventLogsComponent implements OnInit {
	eventsDateFilterFromURL: { startDate: string | Date; endDate: string | Date } = { startDate: '', endDate: '' };
	eventLogsTableHead: string[] = ['Event Type', 'Endpoint Name', 'Time Created', '', ''];
	dateOptions = ['Last Year', 'Last Month', 'Last Week', 'Yesterday'];
	eventsSearchString?: string;
	eventEndpoint?: string;
	eventSource?: string;
	showEventFilterCalendar: boolean = false;
	isloadingEvents: boolean = false;
	selectedEventsDateOption: string = '';
	eventDetailsTabs = [
		{ id: 'data', label: 'Event' },
		{ id: 'response', label: 'Response' },
		{ id: 'request', label: 'Request' }
	];
	displayedEvents?: {
		date: string;
		content: EVENT[];
	}[];
	events?: { pagination: PAGINATION; content: EVENT[] };
	eventDetailsActiveTab = 'data';
	eventsDetailsItem: any;
	sidebarEventDeliveries!: EVENT_DELIVERY[];
	eventsTimeFilterData: { startTime: string; endTime: string } = { startTime: 'T00:00:00', endTime: 'T23:59:59' };
	@ViewChild('timeFilter', { static: true }) timeFilter!: TimePickerComponent;
	@ViewChild('datePicker', { static: true }) datePicker!: DatePickerComponent;
	@ViewChild('eventsEndpointFilter', { static: true }) eventsEndpointFilter!: ElementRef;
	eventsEndpointFilter$!: Observable<ENDPOINT[]>;
	portalToken = this.route.snapshot.params?.token;
	filterSources: SOURCE[] = [];
	isLoadingSidebarDeliveries = false;
	showBatchRetryModal = false;
	fetchingCount = false;
	isRetrying = false;
	batchRetryCount: any;

	constructor(private eventsLogService: EventLogsService, private generalService: GeneralService, public route: ActivatedRoute, private router: Router, public privateService: PrivateService) {}

	async ngOnInit() {
		this.getFiltersFromURL();
		this.getEvents();
		if (!this.portalToken) this.getSourcesForFilter();
	}

	ngAfterViewInit() {
		if (!this.portalToken) {
			this.eventsEndpointFilter$ = fromEvent<any>(this.eventsEndpointFilter?.nativeElement, 'keyup').pipe(
				map(event => event.target.value),
				startWith(''),
				debounceTime(500),
				distinctUntilChanged(),
				switchMap(search => this.getEndpointsForFilter(search))
			);
		}
	}

	clearEventFilters(filterType?: 'eventsDate' | 'eventsEndpoint' | 'eventsSearch' | 'eventsSource') {
		const activeFilters = Object.assign({}, this.route.snapshot.queryParams);
		let filterItems: string[] = [];
		this.datePicker.clearDate();
		this.timeFilter.filterStartHour = 0;
		this.timeFilter.filterStartMinute = 0;
		this.timeFilter.filterEndHour = 23;
		this.timeFilter.filterEndMinute = 59;

		switch (filterType) {
			case 'eventsEndpoint':
				filterItems = ['eventsEndpoint'];
				break;
			case 'eventsDate':
				filterItems = ['eventsStartDate', 'eventsEndDate'];
				break;
			case 'eventsSearch':
				filterItems = ['eventsSearch'];
				break;
			case 'eventsSource':
				filterItems = ['eventsSource'];
				break;
			default:
				filterItems = ['eventsStartDate', 'eventsEndDate', 'eventsEndpoint', 'eventsSearch', 'eventsSource'];
				break;
		}

		this.eventsDateFilterFromURL = { startDate: '', endDate: '' };
		this.eventsTimeFilterData = { startTime: 'T00:00:00', endTime: 'T23:59:59' };
		this.eventEndpoint = undefined;
		this.eventSource = undefined;
		this.eventsSearchString = undefined;
		this.timeFilter.clearFilter();

		filterItems.forEach(key => (activeFilters.hasOwnProperty(key) ? delete activeFilters[key] : null));
		this.router.navigate([], { relativeTo: this.route, queryParams: activeFilters });
	}

	async getEndpointsForFilter(search: string): Promise<ENDPOINT[]> {
		return await (
			await this.privateService.getEndpoints({ pageNo: 1, searchString: search })
		).data.content;
	}

	async getSourcesForFilter() {
		try {
			const sourcesResponse = (await this.privateService.getSources()).data.content;
			this.filterSources = sourcesResponse;
		} catch (error) {}
	}

	updateEndpointFilter(endpointId: string, isChecked: any) {
		isChecked.target.checked ? (this.eventEndpoint = endpointId) : (this.eventEndpoint = undefined);
		this.getEvents({ addToURL: true });
	}

	updateSourceFilter(sourceId: string, isChecked: any) {
		isChecked.target.checked ? (this.eventSource = sourceId) : (this.eventSource = undefined);
		this.getEvents({ addToURL: true });
	}

	getSelectedDateRange(dateRange: { startDate: Date; endDate: Date }) {
		this.eventsDateFilterFromURL = { ...dateRange };
		this.getEvents({ addToURL: true });
	}

	getCodeSnippetString(type: 'res_data' | 'header') {
		if (type === 'res_data') {
			if (!this.eventsDetailsItem?.data) return 'No event data was sent';
			return JSON.stringify(this.eventsDetailsItem?.data || this.eventsDetailsItem?.metadata?.data, null, 4).replaceAll(/"([^"]+)":/g, '$1:');
		}
		if (type === 'header') {
			if (!this.eventsDetailsItem?.headers) return 'No event header was sent';
			return JSON.stringify(this.eventsDetailsItem?.headers, null, 4).replaceAll(/"([^"]+)":/g, '$1:');
		}
		return '';
	}

	setDateForFilter(requestDetails: { startDate: any; endDate: any; startTime?: string; endTime?: string }) {
		if (!requestDetails.endDate && !requestDetails.startDate) return { startDate: '', endDate: '' };
		const startDate = requestDetails.startDate ? `${format(requestDetails.startDate, 'yyyy-MM-dd')}${requestDetails?.startTime || 'T00:00:00'}` : '';
		const endDate = requestDetails.endDate ? `${format(requestDetails.endDate, 'yyyy-MM-dd')}${requestDetails?.endTime || 'T23:59:59'}` : '';
		return { startDate, endDate };
	}

	setTimeFilterData(dates: { startDate: string; endDate: string }): { startTime: string; endTime: string } {
		const response = { startTime: '', endTime: '' };
		if (dates.startDate) {
			const hour = new Date(dates.startDate).getHours();
			const minute = new Date(dates.startDate).getMinutes();
			this.timeFilter.filterStartHour = hour;
			this.timeFilter.filterStartMinute = minute;
			response.startTime = `T${hour < 10 ? '0' + hour : hour}:${minute < 10 ? '0' + minute : minute}:00`;
		} else {
			response.startTime = 'T00:00:00';
		}

		if (dates.endDate) {
			const hour = new Date(dates.endDate).getHours();
			const minute = new Date(dates.endDate).getMinutes();
			this.timeFilter.filterEndHour = hour;
			this.timeFilter.filterEndMinute = minute;
			response.endTime = `T${hour < 10 ? '0' + hour : hour}:${minute < 10 ? '0' + minute : minute}:59`;
		} else {
			response.endTime = 'T23:59:59';
		}

		return response;
	}

	// fetch filters from url
	getFiltersFromURL() {
		const filters = this.route.snapshot.queryParams;
		if (Object.keys(filters).length == 0) return;

		this.eventsDateFilterFromURL = { startDate: filters.eventsStartDate ? new Date(filters.eventsStartDate) : '', endDate: filters.eventsEndDate ? new Date(filters.eventsEndDate) : '' };
		if (!this.portalToken) this.eventEndpoint = filters.eventsEndpoint ?? undefined;
		this.eventsSearchString = filters.eventsSearch ?? undefined;
		const eventsTimeFilter = this.setTimeFilterData({ startDate: filters?.eventsStartDate, endDate: filters?.eventsEndDate });
		this.eventsTimeFilterData = { ...eventsTimeFilter };
	}

	addFilterToURL() {
		const currentURLfilters = this.route.snapshot.queryParams;
		const queryParams: any = {};

		const { startDate, endDate } = this.setDateForFilter({ ...this.eventsDateFilterFromURL, ...this.eventsTimeFilterData });
		if (startDate) queryParams.eventsStartDate = startDate;
		if (endDate) queryParams.eventsEndDate = endDate;
		if (this.eventEndpoint) queryParams.eventsEndpoint = this.eventEndpoint;
		queryParams.eventsSource = this.eventSource;
		queryParams.eventsSearch = this.eventsSearchString;

		this.router.navigate([], { queryParams: Object.assign({}, currentURLfilters, queryParams) });
	}

	async getEvents(requestDetails?: { endpointId?: string; addToURL?: boolean; page?: number }): Promise<HTTP_RESPONSE> {
		this.isloadingEvents = true;

		const page = requestDetails?.page || this.route.snapshot.queryParams.page || 1;
		if (page <= 1) {
			delete this.eventsDetailsItem;
			this.sidebarEventDeliveries = [];
		}

		if (requestDetails?.endpointId) this.eventEndpoint = requestDetails.endpointId;
		if (requestDetails?.addToURL) this.addFilterToURL();

		if (this.eventsSearchString) this.displayedEvents = [];
		const { startDate, endDate } = this.setDateForFilter({ ...this.eventsDateFilterFromURL, ...this.eventsTimeFilterData });

		try {
			const eventsResponse = await this.eventsLogService.getEvents({
				pageNo: page,
				startDate,
				endDate,
				endpointId: this.eventEndpoint || '',
				sourceId: this.eventSource || '',
				query: this.eventsSearchString || '',
				token: this.portalToken
			});
			this.events = eventsResponse.data;

			this.displayedEvents = await this.generalService.setContentDisplayed(eventsResponse.data.content);

			// to show endpoint name or source name on events table header
			if (this.displayedEvents && this.displayedEvents.length > 0 && this.displayedEvents[0].content[0].source_metadata?.name) this.eventLogsTableHead[1] = 'Source Name';

			this.eventsDetailsItem = this.events?.content[0];
			this.getEventDeliveriesForSidebar(this.eventsDetailsItem.uid);

			this.isloadingEvents = false;
			return eventsResponse;
		} catch (error: any) {
			this.isloadingEvents = false;
			return error;
		}
	}

	async getEventDeliveriesForSidebar(eventId: string) {
		this.isLoadingSidebarDeliveries = true;
		this.sidebarEventDeliveries = [];

		try {
			const response = await this.eventsLogService.getEventDeliveries({
				eventId,
				startDate: '',
				endDate: '',
				pageNo: 1,
				endpointId: '',
				statusQuery: '',
				token: this.portalToken
			});
			this.sidebarEventDeliveries = response.data.content;
			this.isLoadingSidebarDeliveries = false;

			return;
		} catch (error) {
			this.isLoadingSidebarDeliveries = false;
			return error;
		}
	}

	async fetchRetryCount() {
		const { startDate, endDate } = this.setDateForFilter(this.eventsDateFilterFromURL);
		const page = this.route.snapshot.queryParams.page || 1;
		this.fetchingCount = true;
		try {
			const response = await this.eventsLogService.getRetryCount({
				pageNo: page,
				startDate: startDate,
				endDate: endDate,
				endpointId: this.eventEndpoint || '',
				sourceId: this.eventSource || '',
				token: this.portalToken
			});

			this.batchRetryCount = response.data.num;
			this.fetchingCount = false;
			this.showBatchRetryModal = true;
		} catch (error) {
			this.fetchingCount = false;
		}
	}

	async retryEvent(requestDetails: { e: any; index: number; eventId: string }) {
		requestDetails.e.stopPropagation();

		try {
			const response = await this.eventsLogService.retryEvent({ eventId: requestDetails.eventId, token: this.portalToken });
			this.generalService.showNotification({ message: response.message, style: 'success' });
			this.getEvents();
		} catch (error) {
			return error;
		}
	}

	async batchRetryEvent() {
		const { startDate, endDate } = this.setDateForFilter(this.eventsDateFilterFromURL);
		const page = this.route.snapshot.queryParams.page || 1;
		this.isRetrying = true;

		try {
			const response = await this.eventsLogService.batchRetryEvent({
				pageNo: page || 1,
				startDate: startDate,
				endDate: endDate,
				endpointId: this.eventEndpoint || '',
				sourceId: this.eventSource || '',
				token: this.portalToken
			});

			this.generalService.showNotification({ message: response.message, style: 'success' });
			this.getEvents();
			this.showBatchRetryModal = false;
			this.isRetrying = false;
		} catch (error) {
			this.isRetrying = false;
			return error;
		}
	}

	viewEventDeliveries(eventId: string) {
		this.router.navigate(['/projects/' + this.privateService.activeProjectDetails?.uid + '/events'], { queryParams: { eventId: eventId } });
	}
}
