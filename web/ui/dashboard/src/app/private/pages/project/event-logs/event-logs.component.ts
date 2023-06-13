import { Component, OnInit, ViewChild } from '@angular/core';
import { CommonModule, Location } from '@angular/common';
import { PrivateService } from 'src/app/private/private.service';
import { ActivatedRoute, Router, RouterModule } from '@angular/router';
import { CardComponent } from 'src/app/components/card/card.component';
import { ButtonComponent } from 'src/app/components/button/button.component';
import { CURSOR, PAGINATION } from 'src/app/models/global.model';
import { EmptyStateComponent } from 'src/app/components/empty-state/empty-state.component';
import { TableLoaderModule } from 'src/app/private/components/table-loader/table-loader.module';
import { TagComponent } from 'src/app/components/tag/tag.component';
import { TableComponent, TableCellComponent, TableRowComponent, TableHeadCellComponent, TableHeadComponent } from 'src/app/components/table/table.component';
import { EventLogsService } from './event-logs.service';
import { GeneralService } from 'src/app/services/general/general.service';
import { HTTP_RESPONSE } from 'src/app/models/global.model';
import { format, parseISO } from 'date-fns';
import { SOURCE } from 'src/app/models/source.model';
import { EVENT, EVENT_DELIVERY } from 'src/app/models/event.model';
import { TimePickerComponent } from 'src/app/components/time-picker/time-picker.component';
import { DatePickerComponent } from 'src/app/components/date-picker/date-picker.component';
import { StatusColorModule } from 'src/app/pipes/status-color/status-color.module';
import { PrismModule } from 'src/app/private/components/prism/prism.module';
import { LoaderModule } from 'src/app/private/components/loader/loader.module';
import { FormsModule } from '@angular/forms';
import { DropdownComponent, DropdownOptionDirective } from 'src/app/components/dropdown/dropdown.component';
import { ModalComponent } from 'src/app/components/modal/modal.component';
import { EventsService } from '../events/events.service';
import { PaginationComponent } from 'src/app/private/components/pagination/pagination.component';
import { CopyButtonComponent } from 'src/app/components/copy-button/copy-button.component';
import { ListItemComponent } from 'src/app/components/list-item/list-item.component';

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
		ModalComponent,
		PaginationComponent,
		CopyButtonComponent,
		ListItemComponent,
		DropdownOptionDirective
	],
	templateUrl: './event-logs.component.html',
	styleUrls: ['./event-logs.component.scss']
})
export class EventLogsComponent implements OnInit {
	eventsDateFilterFromURL: { startDate: string; endDate: string } = { startDate: '', endDate: '' };
	eventLogsTableHead: string[] = ['Event ID', 'Source', 'Time', ''];
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
	displayedEvents: { date: string; content: EVENT[] }[] = [];
	events?: { pagination: PAGINATION; content: EVENT[] };
	eventDetailsActiveTab = 'data';
	eventsDetailsItem: any;
	sidebarEventDeliveries: EVENT_DELIVERY[] = [];
	eventsTimeFilterData: { startTime: string; endTime: string } = { startTime: 'T00:00:00', endTime: 'T23:59:59' };
	@ViewChild('datePicker', { static: true }) datePicker!: DatePickerComponent;
	portalToken = this.route.snapshot.params?.token;
	filterSources: SOURCE[] = [];
	isLoadingSidebarDeliveries = true;
	showBatchRetryModal = false;
	fetchingCount = false;
	isRetrying = false;
	batchRetryCount: any;

	constructor(private eventsLogService: EventLogsService, public generalService: GeneralService, public route: ActivatedRoute, private router: Router, public privateService: PrivateService, private eventsService: EventsService, private _location: Location) {}

	async ngOnInit() {
		this.getFiltersFromURL();
		this.getEvents();
		if (!this.portalToken) this.getSourcesForFilter();
	}

	clearEventFilters(filterType?: 'eventsDate' | 'eventsEndpoint' | 'eventsSearch' | 'eventsSource') {
		const activeFilters = Object.assign({}, this.route.snapshot.queryParams);
		let filterItems: string[] = [];
		this.datePicker.clearDate();

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

		filterItems.forEach(key => (activeFilters.hasOwnProperty(key) ? delete activeFilters[key] : null));
		this.router.navigate([], { relativeTo: this.route, queryParams: activeFilters });
		this.getEvents();
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

	updateSourceFilter() {
		this.getEvents({ addToURL: true });
	}

	getSelectedDateRange(dateRange: { startDate: string; endDate: string }) {
		this.eventsDateFilterFromURL = dateRange;
		this.getEvents({ addToURL: true });
	}

	setDateForFilter(requestDetails: { startDate: any; endDate: any; startTime?: string; endTime?: string }) {
		if (!requestDetails.endDate && !requestDetails.startDate) return { startDate: '', endDate: '' };
		const startDate = requestDetails.startDate ? `${format(requestDetails.startDate, 'yyyy-MM-dd')}${requestDetails?.startTime || 'T00:00:00'}` : '';
		const endDate = requestDetails.endDate ? `${format(requestDetails.endDate, 'yyyy-MM-dd')}${requestDetails?.endTime || 'T23:59:59'}` : '';
		return { startDate, endDate };
	}

	// fetch filters from url
	getFiltersFromURL() {
		const filters = this.route.snapshot.queryParams;
		if (Object.keys(filters).length == 0) return;

		this.eventsDateFilterFromURL = { startDate: filters.eventsStartDate || '', endDate: filters.eventsEndDate || '' };
		if (!this.portalToken) this.eventEndpoint = filters.eventsEndpoint ?? undefined;
		this.eventsSearchString = filters.eventsSearch ?? undefined;
		this.eventSource = filters.eventSource;
	}

	addFilterToURL(params?: any) {
		const currentURLfilters = this.route.snapshot.queryParams;
		const queryParams: any = {};

		if (this.eventsDateFilterFromURL.startDate) queryParams.eventsStartDate = this.eventsDateFilterFromURL.startDate;
		if (this.eventsDateFilterFromURL.endDate) queryParams.eventsEndDate = this.eventsDateFilterFromURL.endDate;
		if (this.eventEndpoint) queryParams.eventsEndpoint = this.eventEndpoint;

		queryParams.eventsSource = this.eventSource;
		queryParams.eventsSearch = this.eventsSearchString;

		const paramsObject = Object.assign({}, currentURLfilters, queryParams, params);
		const cleanedQuery: any = Object.fromEntries(Object.entries(paramsObject).filter(([_, q]) => q !== '' && q !== undefined && q !== null));
		const queryParamss = new URLSearchParams(cleanedQuery).toString();
		this._location.go(`${location.pathname}?${queryParamss}`);
	}

	async getEvents(requestDetails?: { endpointId?: string; addToURL?: boolean }, pagination?: { next_page_cursor?: string; prev_page_cursor?: string; direction?: 'next' | 'prev' }): Promise<HTTP_RESPONSE> {
		this.isloadingEvents = true;

		if (requestDetails?.endpointId) this.eventEndpoint = requestDetails.endpointId;
		if (requestDetails?.addToURL) this.addFilterToURL();

		if (!pagination) {
			pagination = { next_page_cursor: 'FFFFFFFF-FFFF-FFFF-FFFF-FFFFFFFFFFFF' };
			delete this.eventsDetailsItem;
			this.sidebarEventDeliveries = [];
		}

		if (this.eventsSearchString) this.displayedEvents = [];

		try {
			const eventsResponse = await this.eventsService.getEvents({
				startDate: this.eventsDateFilterFromURL.startDate,
				endDate: this.eventsDateFilterFromURL.endDate,
				endpointId: this.eventEndpoint || '',
				sourceId: this.eventSource || '',
				query: this.eventsSearchString || '',
				...pagination
			});
			this.events = eventsResponse.data;

			this.displayedEvents = await this.generalService.setContentDisplayed(eventsResponse.data.content);

			this.eventsDetailsItem = this.events?.content[0];
			this.eventsDetailsItem?.uid ? this.getEventDeliveriesForSidebar(this.eventsDetailsItem.uid) : (this.isLoadingSidebarDeliveries = false);

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
			const response = await this.eventsService.getEventDeliveries({ eventId });
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
				page: page,
				startDate: startDate,
				endDate: endDate,
				endpointId: this.eventEndpoint || '',
				sourceId: this.eventSource || ''
			});

			this.batchRetryCount = response.data.num;
			this.fetchingCount = false;
			this.showBatchRetryModal = true;
		} catch (error) {
			this.fetchingCount = false;
		}
	}

	async retryEvent(requestDetails: { eventId: string }) {
		try {
			const response = await this.eventsLogService.retryEvent({ eventId: requestDetails.eventId });
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
				page: page || 1,
				startDate: startDate,
				endDate: endDate,
				endpointId: this.eventEndpoint || '',
				sourceId: this.eventSource || ''
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

	viewEndpoint(endpointId?: string) {
		if (!endpointId || this.portalToken) return;
		this.router.navigate([`/projects/${this.privateService.activeProjectDetails?.uid}/endpoints/${endpointId}`]);
	}

	viewSource(sourceId?: string) {
		if (!sourceId || this.portalToken) return;
		this.router.navigate([`/projects/${this.privateService.activeProjectDetails?.uid}/sources/${sourceId}`]);
	}

	viewEventDeliveries(eventId: string) {
		this.router.navigate([`/projects/${this.privateService.activeProjectDetails?.uid}/events`], { queryParams: { eventId: eventId } });
	}

	paginateEvents(event: CURSOR) {
		this.addFilterToURL(event);
		this.getEvents({}, event);
	}
}
