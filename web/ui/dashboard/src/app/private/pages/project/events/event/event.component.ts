import { DatePipe } from '@angular/common';
import { Component, ElementRef, EventEmitter, Input, OnInit, Output, ViewChild } from '@angular/core';
import { FormBuilder, FormGroup } from '@angular/forms';
import { ActivatedRoute, Router } from '@angular/router';
import { format } from 'date-fns';
import { fromEvent, Observable } from 'rxjs';
import { debounceTime, distinctUntilChanged, map, startWith, switchMap } from 'rxjs/operators';
import { APP } from 'src/app/models/app.model';
import { EVENT, EVENT_DELIVERY } from 'src/app/models/event.model';
import { PAGINATION } from 'src/app/models/global.model';
import { HTTP_RESPONSE } from 'src/app/models/http.model';
import { TimeFilterComponent } from 'src/app/private/components/time-filter/time-filter.component';
import { GeneralService } from 'src/app/services/general/general.service';
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
	eventsFilterDateRange: FormGroup = this.formBuilder.group({
		startDate: [{ value: '', disabled: true }],
		endDate: [{ value: '', disabled: true }]
	});
	eventsTableHead: string[] = ['Event Type', 'App Name', 'Time Created', ''];
	dateOptions = ['Last Year', 'Last Month', 'Last Week', 'Yesterday'];
	eventsSearchString!: string;
	eventApp!: string;
	showEventFilterCalendar: boolean = false;
	showOverlay: boolean = false;
	showEventsAppsDropdown: boolean = false;
	isloadingEvents: boolean = false;
	selectedEventsDateOption: string = '';
	eventDeliveryFilteredByEventId!: string;
	filteredApps!: APP[];
	eventDetailsTabs = [
		{ id: 'data', label: 'Event' },
		{ id: 'response', label: 'Response' },
		{ id: 'request', label: 'Request' }
	];
	displayedEvents: {
		date: string;
		content: EVENT[];
	}[] = [
		{
			date: '28 Mar, 2022',
			content: [
				{
					app_metadata: {
						group_id: 'db78d6fe-b05e-476d-b908-cb6fff26a3ed',
						support_email: 'pelumi@mailinator.com',
						title: 'App B',
						uid: '73bd4f0e-e987-45b6-bf10-2d6da4ad3fe7'
					},
					created_at: '2022-03-28T16:22:51.972Z',
					data: '{test}',
					event_type: 'test.create',
					matched_endpoints: 1,
					provider_id: '73bd4f0e-e987-45b6-bf10-2d6da4ad3fe7',
					uid: 'a4495e71-1747-4869-842b-4bed9fb27f47',
					updated_at: '2022-03-28T16:22:51.972Z'
				}
			]
		}
	];
	events!: { pagination: PAGINATION; content: EVENT[] };
	eventDetailsActiveTab = 'data';
	eventsDetailsItem: any;
	sidebarEventDeliveries!: EVENT_DELIVERY[];
	eventsTimeFilterData: { startTime: string; endTime: string } = { startTime: 'T00:00:00', endTime: 'T23:59:59' };
	eventDelsTimeFilterData: { startTime: string; endTime: string } = { startTime: 'T00:00:00', endTime: 'T23:59:59' };
	@ViewChild('eventsTimeFilter', { static: true }) eventsTimerFilter!: TimeFilterComponent;
	@ViewChild('eventsAppsFilter', { static: true }) eventsAppsFilter!: ElementRef;
	eventsAppsFilter$!: Observable<APP[]>;

	constructor(
		private formBuilder: FormBuilder,
		private eventsService: EventsService,
		private datePipe: DatePipe,
		private generalService: GeneralService,
		private route: ActivatedRoute,
		private router: Router
	) {}

	async ngOnInit() {
		this.getFiltersFromURL();
		await this.getEvents();
	}

	ngAfterViewInit() {
		this.eventsAppsFilter$ = fromEvent<any>(this.eventsAppsFilter?.nativeElement, 'keyup').pipe(
			map(event => event.target.value),
			startWith(''),
			debounceTime(500),
			distinctUntilChanged(),
			switchMap(search => this.getAppsForFilter(search))
		);
	}

	async clearEventFilters(filterType?: 'eventsDate' | 'eventsApp' | 'eventsSearch') {
		const activeFilters = Object.assign({}, this.route.snapshot.queryParams);
		let filterItems: string[] = [];

		this.eventApp = '';
		this.eventsFilterDateRange.patchValue({ startDate: '', endDate: '' });
		this.eventsSearchString = '';

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
		this.eventsFilterDateRange.patchValue({ startDate: '', endDate: '' });
		this.eventsTimeFilterData = { startTime: 'T00:00:00', endTime: 'T23:59:59' };
		this.eventsTimerFilter.clearFilter();
		this.getEvents();

		filterItems.forEach(key => (activeFilters.hasOwnProperty(key) ? delete activeFilters[key] : null));
		await this.router.navigate([], { relativeTo: this.route, queryParams: activeFilters });
	}

	async getAppsForFilter(search: string): Promise<APP[]> {
		return await (
			await this.eventsService.getApps({ pageNo: 1, searchString: search })
		).data.content;
	}

	updateAppFilter(appId: string, isChecked: any) {
		this.showOverlay = false;
		this.showEventsAppsDropdown = !this.showEventsAppsDropdown;
		isChecked.target.checked ? (this.eventApp = appId) : (this.eventApp = '');

		this.getEvents({ addToURL: true });
	}

	getSelectedDateRange(dateRange: { startDate: Date; endDate: Date }) {
		this.eventsFilterDateRange.patchValue({
			startDate: dateRange.startDate,
			endDate: dateRange.endDate
		});
		this.getEvents({ addToURL: true });
	}

	getCodeSnippetString() {
		if (!this.eventsDetailsItem?.data) return 'No event data was sent';
		return JSON.stringify(this.eventsDetailsItem?.data || this.eventsDetailsItem?.metadata?.data, null, 4).replaceAll(/"([^"]+)":/g, '$1:');
	}

	setDateForFilter(requestDetails: { startDate: Date; endDate: Date; startTime?: string; endTime?: string }) {
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
			this.eventsTimerFilter.filterStartHour = hour;
			this.eventsTimerFilter.filterStartMinute = minute;

			response.startTime = `T${hour}:${minute}:00`;
		} else {
			response.startTime = 'T00:00:00';
		}

		if (dates.endDate) {
			const hour = new Date(dates.endDate).getHours();
			const minute = new Date(dates.endDate).getMinutes();
			this.eventsTimerFilter.filterEndHour = hour;
			this.eventsTimerFilter.filterEndMinute = minute;
			response.endTime = `T${hour}:${minute}:59`;
		} else {
			response.endTime = 'T23:59:59';
		}

		return response;
	}
	// fetch filters from url
	getFiltersFromURL() {
		const filters = this.route.snapshot.queryParams;
		if (Object.keys(filters).length == 0) return;

		this.eventsFilterDateRange.patchValue({ startDate: filters.eventsStartDate ? new Date(filters.eventsStartDate) : '', endDate: filters.eventsEndDate ? new Date(filters.eventsEndDate) : '' });
		this.eventApp = filters.eventsApp ?? '';
		this.eventsSearchString = filters.eventsSearch ?? '';
		const eventsTimeFilter = this.setTimeFilterData({ startDate: filters?.eventsStartDate, endDate: filters?.eventsEndDate });
		this.eventsTimeFilterData = { ...eventsTimeFilter };
	}
	addFilterToURL() {
		const currentURLfilters = this.route.snapshot.queryParams;
		const queryParams: any = {};

		const { startDate, endDate } = this.setDateForFilter({ ...this.eventsFilterDateRange.value, ...this.eventsTimeFilterData });
		if (startDate) queryParams.eventsStartDate = startDate;
		if (endDate) queryParams.eventsEndDate = endDate;
		if (this.eventApp) queryParams.eventsApp = this.eventApp;
		if (this.eventsSearchString) queryParams.eventsSearch = this.eventsSearchString;

		this.router.navigate([], { queryParams: Object.assign({}, currentURLfilters, queryParams) });
	}

	async getEvents(requestDetails?: { appId?: string; addToURL?: boolean; page?: number }): Promise<HTTP_RESPONSE> {
		const page = requestDetails?.page || this.route.snapshot.queryParams.page || 1;
		if (requestDetails?.appId) this.eventApp = requestDetails.appId;
		if (requestDetails?.addToURL) this.addFilterToURL();

		if (this.eventsSearchString) this.displayedEvents = [];
		console.log(this.eventApp);
		const { startDate, endDate } = this.setDateForFilter({ ...this.eventsFilterDateRange.value, ...this.eventsTimeFilterData });

		try {
			const eventsResponse = await this.eventsService.getEvents({
				pageNo: page,
				startDate,
				endDate,
				appId: this.eventApp || '',
				query: this.eventsSearchString || ''
			});

			this.events = eventsResponse.data;
			this.eventsDetailsItem = this.events?.content[0];
			this.getEventDeliveriesForSidebar(this.eventsDetailsItem.uid);
			this.displayedEvents = await this.generalService.setContentDisplayed(eventsResponse.data.content);
			this.pushEvents.emit(this.events);
			this.isloadingEvents = false;
			return eventsResponse;
		} catch (error: any) {
			this.isloadingEvents = false;
			return error;
		}
	}

	async getEventDeliveriesForSidebar(eventId: string) {
		const response = await this.eventsService.getEventDeliveries({
			eventId,
			startDate: '',
			endDate: '',
			pageNo: 1,
			appId: '',
			statusQuery: ''
		});
		this.sidebarEventDeliveries = response.data.content;
	}

	openDeliveriesTab(eventId: string) {
		this.getEventDeliveries.emit(eventId);
	}
}
