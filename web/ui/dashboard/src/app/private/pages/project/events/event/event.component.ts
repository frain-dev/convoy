import { Component, ElementRef, EventEmitter, Input, OnInit, Output, ViewChild } from '@angular/core';
import { ActivatedRoute, Router } from '@angular/router';
import { format } from 'date-fns';
import { fromEvent, Observable } from 'rxjs';
import { debounceTime, distinctUntilChanged, map, startWith, switchMap } from 'rxjs/operators';
import { APP } from 'src/app/models/app.model';
import { EVENT, EVENT_DELIVERY } from 'src/app/models/event.model';
import { PAGINATION } from 'src/app/models/global.model';
import { HTTP_RESPONSE } from 'src/app/models/http.model';
import { DateFilterComponent } from 'src/app/private/components/date-filter/date-filter.component';
import { TimeFilterComponent } from 'src/app/private/components/time-filter/time-filter.component';
import { GeneralService } from 'src/app/services/general/general.service';
import { DropdownComponent } from 'src/stories/dropdown/dropdown.component';
import { EventsService } from '../events.service';

@Component({
	selector: 'app-event',
	templateUrl: './event.component.html',
	styleUrls: ['../events.component.scss']
})
export class EventComponent implements OnInit {
	@Input() activeTab!: string;
	@Output() getEventDeliveries = new EventEmitter<string>();
	@Output() pushEvents = new EventEmitter<any>();
	eventsDateFilterFromURL: { startDate: string | Date; endDate: string | Date } = { startDate: '', endDate: '' };
	eventsTableHead: string[] = ['Event Type', 'App Name', 'Time Created', ''];
	dateOptions = ['Last Year', 'Last Month', 'Last Week', 'Yesterday'];
	eventsSearchString?: string;
	eventApp?: string;
	showEventFilterCalendar: boolean = false;
	showOverlay: boolean = false;
	showEventsAppsDropdown: boolean = false;
	isloadingEvents: boolean = false;
	selectedEventsDateOption: string = '';
	eventDetailsTabs = [
		{ id: 'data', label: 'Event' },
		{ id: 'response', label: 'Response' },
		{ id: 'request', label: 'Request' }
	];
	displayedEvents!: {
		date: string;
		content: EVENT[];
	}[];
	events!: { pagination: PAGINATION; content: EVENT[] };
	eventDetailsActiveTab = 'data';
	eventsDetailsItem: any;
	sidebarEventDeliveries!: EVENT_DELIVERY[];
	eventsTimeFilterData: { startTime: string; endTime: string } = { startTime: 'T00:00:00', endTime: 'T23:59:59' };
	@ViewChild('timeFilter', { static: true }) timeFilter!: TimeFilterComponent;
	@ViewChild('dateFilter', { static: true }) dateFilter!: DateFilterComponent;
	@ViewChild('eventsAppsFilter', { static: true }) eventsAppsFilter!: ElementRef;
	@ViewChild(DropdownComponent) appDropdownComponent!: DropdownComponent;
	eventsAppsFilter$!: Observable<APP[]>;
	appPortalToken = this.route.snapshot.params?.token;

	constructor(private eventsService: EventsService, private generalService: GeneralService, private route: ActivatedRoute, private router: Router) {}

	async ngOnInit() {
		this.getFiltersFromURL();
		this.getEvents();
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

	clearEventFilters(filterType?: 'eventsDate' | 'eventsApp' | 'eventsSearch') {
		const activeFilters = Object.assign({}, this.route.snapshot.queryParams);
		let filterItems: string[] = [];
		this.appDropdownComponent.show = false;
		this.dateFilter.clearDate();
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
			default:
				filterItems = ['eventsStartDate', 'eventsEndDate', 'eventsApp', 'eventsSearch'];
				break;
		}

		this.eventsDateFilterFromURL = { startDate: '', endDate: '' };
		this.eventsTimeFilterData = { startTime: 'T00:00:00', endTime: 'T23:59:59' };
		this.eventApp = undefined;
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

	updateAppFilter(appId: string, isChecked: any) {
		this.showOverlay = false;
		this.showEventsAppsDropdown = !this.showEventsAppsDropdown;
		isChecked.target.checked ? (this.eventApp = appId) : (this.eventApp = undefined);

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
				query: this.eventsSearchString || '',
				token: this.appPortalToken
			});
			this.events = eventsResponse.data;
			this.displayedEvents = await this.generalService.setContentDisplayed(eventsResponse.data.content);
			this.eventsDetailsItem = this.events?.content[0];
			this.getEventDeliveriesForSidebar(this.eventsDetailsItem.uid);

			this.pushEvents.emit(this.events);
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
