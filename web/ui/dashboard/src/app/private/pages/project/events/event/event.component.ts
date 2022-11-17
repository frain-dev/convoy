import { Component, ElementRef, EventEmitter, Input, OnInit, Output, ViewChild } from '@angular/core';
import { ActivatedRoute, Router } from '@angular/router';
import { format } from 'date-fns';
import { fromEvent, Observable } from 'rxjs';
import { debounceTime, distinctUntilChanged, map, startWith, switchMap } from 'rxjs/operators';
import { APP } from 'src/app/models/app.model';
import { EVENT, EVENT_DELIVERY } from 'src/app/models/event.model';
import { PAGINATION } from 'src/app/models/global.model';
import { HTTP_RESPONSE } from 'src/app/models/http.model';
import { GeneralService } from 'src/app/services/general/general.service';
import { EventsService } from '../events.service';
import { PrivateService } from 'src/app/private/private.service';
import { SOURCE } from 'src/app/models/group.model';
import { DatePickerComponent } from 'src/app/components/date-picker/date-picker.component';
import { TimePickerComponent } from 'src/app/components/time-picker/time-picker.component';

@Component({
	selector: 'app-event',
	templateUrl: './event.component.html',
	styleUrls: ['../events.component.scss']
})
export class EventComponent implements OnInit {
	@Input() activeTab!: string;
	@Output() getEventDeliveries = new EventEmitter<string>();
	@Output() pushEvents = new EventEmitter<any>();
	@Output() openSource = new EventEmitter<string>();
	@Output() openApp = new EventEmitter<string>();
	eventsDateFilterFromURL: { startDate: string | Date; endDate: string | Date } = { startDate: '', endDate: '' };
	eventsTableHead: string[] = ['Event Type', 'App Name', 'Time Created', ''];
	dateOptions = ['Last Year', 'Last Month', 'Last Week', 'Yesterday'];
	eventsSearchString?: string;
	eventApp?: string;
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
	@ViewChild('eventsAppsFilter', { static: true }) eventsAppsFilter!: ElementRef;
	eventsAppsFilter$!: Observable<APP[]>;
	appPortalToken = this.route.snapshot.params?.token;
	filterSources: SOURCE[] = [];

	constructor(private eventsService: EventsService, private generalService: GeneralService, private route: ActivatedRoute, private router: Router, public privateService: PrivateService) {}

	async ngOnInit() {
		this.getFiltersFromURL();
		this.getEvents();
		if (!this.appPortalToken) this.getSourcesForFilter();
	}

	ngAfterViewInit() {
		if (!this.appPortalToken) {
			this.eventsAppsFilter$ = fromEvent<any>(this.eventsAppsFilter?.nativeElement, 'keyup').pipe(
				map(event => event.target.value),
				startWith(''),
				debounceTime(500),
				distinctUntilChanged(),
				switchMap(search => this.getAppsForFilter(search))
			);
		}
	}

	clearEventFilters(filterType?: 'eventsDate' | 'eventsApp' | 'eventsSearch' | 'eventsSource') {
		const activeFilters = Object.assign({}, this.route.snapshot.queryParams);
		let filterItems: string[] = [];
		this.datePicker.clearDate();
		this.timeFilter.filterStartHour = 0;
		this.timeFilter.filterStartMinute = 0;
		this.timeFilter.filterEndHour = 23;
		this.timeFilter.filterEndMinute = 59;

		switch (filterType) {
			case 'eventsApp':
				filterItems = ['eventsApp'];
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
				filterItems = ['eventsStartDate', 'eventsEndDate', 'eventsApp', 'eventsSearch', 'eventsSource'];
				break;
		}

		this.eventsDateFilterFromURL = { startDate: '', endDate: '' };
		this.eventsTimeFilterData = { startTime: 'T00:00:00', endTime: 'T23:59:59' };
		this.eventApp = undefined;
		this.eventSource = undefined;
		this.eventsSearchString = undefined;
		this.timeFilter.clearFilter();

		filterItems.forEach(key => (activeFilters.hasOwnProperty(key) ? delete activeFilters[key] : null));
		this.router.navigate([], { relativeTo: this.route, queryParams: activeFilters });
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
		isChecked.target.checked ? (this.eventApp = appId) : (this.eventApp = undefined);
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

	getCodeSnippetString() {
		if (!this.eventsDetailsItem?.data) return 'No event data was sent';
		return JSON.stringify(this.eventsDetailsItem?.data || this.eventsDetailsItem?.metadata?.data, null, 4).replaceAll(/"([^"]+)":/g, '$1:');
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
		if (!this.appPortalToken) this.eventApp = filters.eventsApp ?? undefined;
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
		if (this.eventApp) queryParams.eventsApp = this.eventApp;
		queryParams.eventsSource = this.eventSource;
		queryParams.eventsSearch = this.eventsSearchString;

		this.router.navigate([], { queryParams: Object.assign({}, currentURLfilters, queryParams) });
	}

	async getEvents(requestDetails?: { appId?: string; addToURL?: boolean; page?: number }): Promise<HTTP_RESPONSE> {
		this.isloadingEvents = true;

		const page = requestDetails?.page || this.route.snapshot.queryParams.page || 1;
		if (page <= 1) {
			delete this.eventsDetailsItem;
			this.sidebarEventDeliveries = [];
		}

		if (requestDetails?.appId) this.eventApp = requestDetails.appId;
		if (requestDetails?.addToURL) this.addFilterToURL();

		if (this.eventsSearchString) this.displayedEvents = [];
		const { startDate, endDate } = this.setDateForFilter({ ...this.eventsDateFilterFromURL, ...this.eventsTimeFilterData });

		try {
			const eventsResponse = await this.eventsService.getEvents({
				pageNo: page,
				startDate,
				endDate,
				appId: this.eventApp || '',
				sourceId: this.eventSource || '',
				query: this.eventsSearchString || '',
				token: this.appPortalToken
			});
			this.events = eventsResponse.data;

			this.displayedEvents = await this.generalService.setContentDisplayed(eventsResponse.data.content);

			// to show app name or source name on events table header
			if (this.displayedEvents && this.displayedEvents.length > 0 && this.displayedEvents[0].content[0].source_metadata?.name) this.eventsTableHead[1] = 'Source Name';

			this.pushEvents.emit(this.events);
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
		this.sidebarEventDeliveries = [];

		const response = await this.eventsService.getEventDeliveries({
			eventId,
			startDate: '',
			endDate: '',
			pageNo: 1,
			appId: '',
			statusQuery: '',
			token: this.appPortalToken
		});
		this.sidebarEventDeliveries = response.data.content;
		return;
	}

	openDeliveriesTab(eventId: string) {
		this.getEventDeliveries.emit(eventId);
	}
}
