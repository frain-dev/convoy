import { Component, ElementRef, Input, OnInit, ViewChild } from '@angular/core';
import Chart from 'chart.js/auto';
import { APP } from './models/app.model';
import { EVENT, EVENT_DELIVERY, EVENT_DELIVERY_ATTEMPT } from './models/event.model';
import { ActivatedRoute, Router } from '@angular/router';
import { FormBuilder, FormGroup } from '@angular/forms';
import { PAGINATION } from './models/global.model';
import { HTTP_RESPONSE } from './models/http.model';
import { GROUP } from './models/group.model';
import { ConvoyDashboardService } from './convoy-dashboard.service';
import { format } from 'date-fns';
import { fromEvent, Observable } from 'rxjs';
import { debounceTime, distinctUntilChanged, map, startWith, switchMap } from 'rxjs/operators';

@Component({
	selector: 'convoy-dashboard',
	templateUrl: './convoy-dashboard.component.html',
	styleUrls: ['./convoy-dashboard.component.scss']
})
export class ConvoyDashboardComponent implements OnInit {
	showFilterCalendar = false;
	tabs: ['events', 'event deliveries', 'apps'] = ['events', 'event deliveries', 'apps'];
	activeTab: 'events' | 'apps' | 'event deliveries' = 'events';
	appsDetailsItem?: any;
	eventsDetailsItem?: any;
	eventDelsDetailsItem?: any;
	eventDeliveryAtempt?: EVENT_DELIVERY_ATTEMPT;
	showEventFilterCalendar = false;
	eventDateFilterActive = false;
	displayedEvents: {
		date: string;
		events: EVENT[];
	}[] = [];
	events!: { pagination: PAGINATION; content: EVENT[] };
	apps!: { pagination: PAGINATION; content: APP[] };
	filteredApps!: APP[];
	eventDetailsTabs = [
		{ id: 'data', label: 'Event' },
		{ id: 'response', label: 'Response' },
		{ id: 'request', label: 'Request' }
	];
	eventDetailsActiveTab = 'data';
	organisationDetails!: {
		database: { dsn: string };
		queue: { type: string; redis: { dsn: string } };
		server: { http: { port: number } };
		signature: { header: string; hash: string };
		strategy: { type: 'default'; default: { intervalSeconds: number; retryLimit: number } };
	};
	dashboardData = { apps: 0, events_sent: 0 };
	eventApp: string = '';
	eventDeliveriesApp: string = '';
	eventsPage: number = 1;
	eventDeliveriesPage: number = 1;
	appsPage: number = 1;
	dashboardFrequency: 'daily' | 'weekly' | 'monthly' | 'yearly' = 'daily';
	statsDateRange: FormGroup = this.formBuilder.group({
		startDate: [{ value: new Date(new Date().setDate(new Date().getDate() - 30)), disabled: true }],
		endDate: [{ value: new Date(), disabled: true }]
	});
	eventsFilterDateRange: FormGroup = this.formBuilder.group({
		startDate: [{ value: '', disabled: true }],
		endDate: [{ value: '', disabled: true }]
	});
	eventDeliveriesFilterDateRange: FormGroup = this.formBuilder.group({
		startDate: [{ value: '', disabled: true }],
		endDate: [{ value: '', disabled: true }]
	});
	selectedEventsFromEventDeliveriesTable: string[] = [];
	displayedEventDeliveries: { date: string; events: EVENT_DELIVERY[] }[] = [];
	eventDeliveries!: { pagination: PAGINATION; content: EVENT_DELIVERY[] };
	sidebarEventDeliveries: EVENT_DELIVERY[] = [];
	eventDeliveryFilteredByEventId = '';
	groups: GROUP[] = [];
	activeGroup!: string;
	allEventdeliveriesChecked = false;
	eventDeliveryStatuses = ['Success', 'Failure', 'Retry', 'Scheduled', 'Processing', 'Discarded'];
	eventDeliveryFilteredByStatus: string[] = [];
	showOverlay = false;
	showEventDeliveriesStatusDropdown = false;
	showEventDeliveriesAppsDropdown = false;
	showEventsAppsDropdown = false;
	@Input('apiURL') apiURL: string = '';
	@Input('isCloud') isCloud: boolean = false;
	@Input('groupId') groupId: string = '';
	@Input('requestToken') requestToken: string = '';
	apiAuthType: 'Basic' | 'Bearer' = 'Basic';
	isloadingDashboardData = true;
	isloadingConfig = true;
	isloadingEvents = true;
	isloadingEventDeliveries = true;
	isloadingApps = true;
	isloadingMoreEvents = false;
	isloadingMoreEventDeliveries = false;
	isloadingMoreApps = false;
	isloadingDeliveryAttempt = false;
	appsSearchString = '';
	eventsAppsFilter$!: Observable<APP[]>;
	eventsDelAppsFilter$!: Observable<APP[]>;
	@ViewChild('eventsAppsFilter', { static: true }) eventsAppsFilter!: ElementRef;
	@ViewChild('eventDelsAppsFilter', { static: true }) eventDelsAppsFilter!: ElementRef;
	eventDeliveriesStatusFilterActive = false;

	constructor(private convyDashboardService: ConvoyDashboardService, private router: Router, private formBuilder: FormBuilder, private route: ActivatedRoute) {}

	async ngOnInit() {
		if (!this.requestToken || this.requestToken == '') {
			this.convyDashboardService.showNotification({ message: 'You are not logged in' });
			return this.router.navigate(['/login']);
		}

		if (!this.apiURL) return this.convyDashboardService.showNotification({ message: 'Please provide API URL for Convoy dashboard component.' });
		if (this.isCloud && !this.groupId) return this.convyDashboardService.showNotification({ message: 'Please provide group ID for Convoy dashboard component.' });
		if (this.isCloud) {
			this.activeGroup = this.groupId;
			this.apiAuthType = 'Bearer';
		}

		return await this.initDashboard();
	}

	ngAfterViewInit() {
		this.eventsAppsFilter$ = fromEvent<any>(this.eventsAppsFilter.nativeElement, 'keyup').pipe(
			map(event => event.target.value),
			startWith(''),
			debounceTime(500),
			distinctUntilChanged(),
			switchMap(search => this.getAppsForFilter(search))
		);
		this.eventsDelAppsFilter$ = fromEvent<any>(this.eventDelsAppsFilter.nativeElement, 'keyup').pipe(
			map(event => event.target.value),
			startWith(''),
			debounceTime(500),
			distinctUntilChanged(),
			switchMap(search => this.getAppsForFilter(search))
		);
	}

	async initDashboard() {
		await this.getGroups();
		this.getFiltersFromURL();
		await Promise.all([this.getConfigDetails(), this.fetchDashboardData(), this.getEvents(), this.getApps({ type: 'apps' }), this.getEventDeliveries()]);

		// get active tab from url and apply, after getting the details from above requests so that the data is available ahead
		this.toggleActiveTab(this.route.snapshot.queryParams.activeTab ?? 'events');
		return;
	}

	toggleActiveTab(tab: 'events' | 'apps' | 'event deliveries') {
		this.activeTab = tab;
		this.addFilterToURL({ section: 'logTab' });

		if (tab === 'apps' && this.apps?.content.length > 0) {
			if (!this.appsDetailsItem) this.appsDetailsItem = this.apps?.content[0];
		} else if (tab === 'events' && this.events?.content.length > 0) {
			if (!this.eventsDetailsItem) this.eventsDetailsItem = this.events?.content[0];
			if (this.eventsDetailsItem?.uid) this.getEventDeliveriesForSidebar(this.eventsDetailsItem.uid);
		} else if (tab === 'event deliveries' && this.eventDeliveries?.content.length > 0) {
			if (!this.eventDelsDetailsItem) this.eventDelsDetailsItem = this.eventDeliveries?.content[0];
			if (this.eventDelsDetailsItem?.uid) this.getDelieveryAttempts(this.eventDelsDetailsItem.uid);
		}
	}

	async getConfigDetails() {
		this.isloadingConfig = true;

		try {
			const organisationDetailsResponse = await this.convyDashboardService.request({
				url: this.getAPIURL(`/dashboard/config?groupID=${this.activeGroup || ''}`),
				token: this.requestToken,
				authType: this.apiAuthType,
				method: 'get'
			});
			this.organisationDetails = organisationDetailsResponse.data;
			this.isloadingConfig = false;
		} catch (error) {
			this.isloadingConfig = false;
		}
	}

	getFiltersFromURL() {
		const filters = this.route.snapshot.queryParams;
		if (Object.keys(filters).length == 0) return;

		// for events filters
		this.eventsFilterDateRange.patchValue({ startDate: filters.eventsStartDate ? new Date(filters.eventsStartDate) : '', endDate: filters.eventsEndDate ? new Date(filters.eventsEndDate) : '' });
		this.eventApp = filters.eventsApp ?? '';

		// for event deliveries filters
		this.eventDeliveriesFilterDateRange.patchValue({
			startDate: filters.eventDelsStartDate ? new Date(filters.eventDelsStartDate) : '',
			endDate: filters.eventDelsEndDate ? new Date(filters.eventDelsEndDate) : ''
		});
		this.eventDeliveriesApp = filters.eventDelsApp ?? '';
		this.eventDeliveryFilteredByStatus = filters.eventDelsStatus ? JSON.parse(filters.eventDelsStatus) : [];
	}

	async fetchDashboardData() {
		try {
			this.isloadingDashboardData = true;
			const { startDate, endDate } = this.setDateForFilter(this.statsDateRange.value);

			const dashboardResponse = await this.convyDashboardService.request({
				url: this.getAPIURL(`/dashboard/summary?groupID=${this.activeGroup || ''}&startDate=${startDate || ''}&endDate=${endDate || ''}&type=${this.dashboardFrequency}`),
				token: this.requestToken,
				authType: this.apiAuthType,
				method: 'get'
			});
			this.dashboardData = dashboardResponse.data;
			this.isloadingDashboardData = false;

			let labelsDateFormat = '';
			if (this.dashboardFrequency === 'daily') labelsDateFormat = 'do, MMM';
			else if (this.dashboardFrequency === 'monthly') labelsDateFormat = 'MMM';
			else if (this.dashboardFrequency === 'yearly') labelsDateFormat = 'yyyy';

			const chartData = dashboardResponse.data.event_data;
			const labels = [...chartData.map((label: { data: { date: any } }) => label.data.date)].map(date => {
				return this.dashboardFrequency === 'weekly' ? date : format(new Date(date), labelsDateFormat);
			});
			labels.unshift('0');
			const dataSet = [0, ...chartData.map((label: { count: any }) => label.count)];
			const data = {
				labels,
				datasets: [
					{
						data: dataSet,
						fill: false,
						borderColor: '#477DB3',
						tension: 0.5,
						yAxisID: 'yAxis',
						xAxisID: 'xAxis'
					}
				]
			};

			const options = {
				plugins: {
					legend: {
						display: false
					}
				},
				scales: {
					xAxis: {
						display: true,
						grid: {
							display: false
						}
					}
				}
			};

			if (!Chart.getChart('dahboard_events_chart') || !Chart.getChart('dahboard_events_chart')?.canvas) {
				new Chart('dahboard_events_chart', { type: 'line', data, options });
			} else {
				const currentChart = Chart.getChart('dahboard_events_chart');
				if (currentChart) {
					currentChart.data.labels = labels;
					currentChart.data.datasets[0].data = dataSet;
					currentChart.update();
				}
			}
		} catch (error) {}
	}

	setDateForFilter(requestDetails: { startDate: Date; endDate: Date }) {
		if (!requestDetails.endDate && !requestDetails.startDate) return { startDate: '', endDate: '' };
		const startDate = requestDetails.startDate ? `${format(requestDetails.startDate, 'yyyy-MM-dd')}T00:00:00` : '';
		const endDate = requestDetails.endDate ? `${format(requestDetails.endDate, 'yyyy-MM-dd')}T23:59:59` : '';
		return { startDate, endDate };
	}

	getDate(date: Date) {
		const months = ['Jan', 'Feb', 'Mar', 'April', 'May', 'June', 'July', 'Aug', 'Sept', 'Oct', 'Nov', 'Dec'];
		const _date = new Date(date);
		const day = _date.getDate();
		const month = _date.getMonth();
		const year = _date.getFullYear();
		return `${day} ${months[month]}, ${year}`;
	}

	setEventsDisplayed(events: { created_at: Date }[]) {
		const dateCreateds = events.map((event: { created_at: Date }) => this.getDate(event.created_at));
		const uniqueDateCreateds = [...new Set(dateCreateds)];
		const displayedEvents: any = [];
		uniqueDateCreateds.forEach(eventDate => {
			const filteredEventDate = events.filter((event: { created_at: Date }) => this.getDate(event.created_at) === eventDate);
			const eventsItem = { date: eventDate, events: filteredEventDate };
			displayedEvents.push(eventsItem);
		});
		return displayedEvents;
	}

	async getEvents(requestDetails?: { appId?: string; addToURL?: boolean; fromFilter?: boolean }) {
		this.events && this.events?.pagination?.next === this.eventsPage ? (this.isloadingMoreEvents = true) : (this.isloadingEvents = true);

		if (requestDetails?.appId) this.eventApp = requestDetails.appId;
		if (requestDetails?.addToURL) this.addFilterToURL({ section: 'events' });

		const { startDate, endDate } = this.setDateForFilter(this.eventsFilterDateRange.value);

		try {
			const eventsResponse = await this.convyDashboardService.request({
				url: this.getAPIURL(
					`/events?groupID=${this.activeGroup || ''}&sort=AESC&page=${this.eventsPage || 1}&perPage=20&startDate=${startDate}&endDate=${endDate}&appId=${requestDetails?.appId ?? this.eventApp}`
				),
				token: this.requestToken,
				authType: this.apiAuthType,
				method: 'get'
			});

			if (this.events && this.events?.pagination?.next === this.eventsPage) {
				const content = [...this.events.content, ...eventsResponse.data.content];
				const pagination = eventsResponse.data.pagination;
				this.events = { content, pagination };
				this.displayedEvents = this.setEventsDisplayed(content);
				this.isloadingMoreEvents = false;
				return;
			}

			this.events = eventsResponse.data;
			this.displayedEvents = await this.setEventsDisplayed(eventsResponse.data.content);

			// if this is a filter request, set the eventsDetailsItem to the first item in the list
			if (requestDetails?.fromFilter) {
				this.eventsDetailsItem = this.events?.content[0];
				this.getEventDeliveriesForSidebar(this.eventsDetailsItem.uid);
			}

			this.isloadingEvents = false;
		} catch (error) {
			this.isloadingEvents = false;
			this.isloadingMoreEvents = false;
			return error;
		}
	}

	async loadEventsFromAppsTable(appId: string) {
		await this.getEvents({ addToURL: true, appId: appId, fromFilter: true });
		this.toggleActiveTab('events');
	}

	addFilterToURL(requestDetails: { section: 'events' | 'eventDels' | 'group' | 'logTab' }) {
		const currentURLfilters = this.route.snapshot.queryParams;
		const queryParams: any = {};

		if (requestDetails.section === 'events') {
			const { startDate, endDate } = this.setDateForFilter(this.eventsFilterDateRange.value);
			if (startDate) queryParams.eventsStartDate = startDate;
			if (endDate) queryParams.eventsEndDate = endDate;
			if (this.eventApp) queryParams.eventsApp = this.eventApp;
		}

		if (requestDetails.section === 'eventDels') {
			const { startDate, endDate } = this.setDateForFilter(this.eventDeliveriesFilterDateRange.value);
			if (startDate) queryParams.eventDelsStartDate = startDate;
			if (endDate) queryParams.eventDelsEndDate = endDate;
			if (this.eventDeliveriesApp) queryParams.eventDelsApp = this.eventDeliveriesApp;
			queryParams.eventDelsStatus = this.eventDeliveryFilteredByStatus.length > 0 ? JSON.stringify(this.eventDeliveryFilteredByStatus) : '';
		}

		if (requestDetails.section === 'group') queryParams.group = this.activeGroup;

		if (requestDetails.section === 'logTab') queryParams.activeTab = this.activeTab;

		this.router.navigate([], { queryParams: Object.assign({}, currentURLfilters, queryParams) });
	}

	async eventDeliveriesRequest(requestDetails: { eventId?: string; startDate?: string; endDate?: string }): Promise<HTTP_RESPONSE> {
		let eventDeliveryStatusFilterQuery = '';
		this.eventDeliveryFilteredByStatus.length > 0 ? (this.eventDeliveriesStatusFilterActive = true) : (this.eventDeliveriesStatusFilterActive = false);
		this.eventDeliveryFilteredByStatus.forEach((status: string) => (eventDeliveryStatusFilterQuery += `&status=${status}`));
		const { startDate, endDate } = this.setDateForFilter(this.eventDeliveriesFilterDateRange.value);

		try {
			const eventDeliveriesResponse = await this.convyDashboardService.request({
				url: this.getAPIURL(
					`/eventdeliveries?groupID=${this.activeGroup || ''}&eventId=${requestDetails.eventId || ''}&page=${this.eventDeliveriesPage || 1}&startDate=${startDate}&endDate=${endDate}&appId=${
						this.eventDeliveriesApp
					}${eventDeliveryStatusFilterQuery || ''}`
				),
				token: this.requestToken,
				authType: this.apiAuthType,
				method: 'get'
			});

			return eventDeliveriesResponse;
		} catch (error: any) {
			return error;
		}
	}

	updateEventDevliveryStatusFilter(status: string, isChecked: any) {
		if (isChecked.target.checked) {
			this.eventDeliveryFilteredByStatus.push(status);
		} else {
			let index = this.eventDeliveryFilteredByStatus.findIndex(x => x === status);
			this.eventDeliveryFilteredByStatus.splice(index, 1);
		}
	}

	updateAppFilter(appId: string, isChecked: any, activeSection: 'eventDels' | 'events') {
		this.showOverlay = false;
		activeSection === 'eventDels' ? (this.showEventDeliveriesAppsDropdown = !this.showEventDeliveriesAppsDropdown) : (this.showEventsAppsDropdown = !this.showEventsAppsDropdown);
		if (isChecked.target.checked) {
			activeSection === 'eventDels' ? (this.eventDeliveriesApp = appId) : (this.eventApp = appId);
		} else {
			activeSection === 'eventDels' ? (this.eventDeliveriesApp = '') : (this.eventApp = '');
		}
		activeSection === 'eventDels' ? this.getEventDeliveries({ addToURL: true, fromFilter: true }) : this.getEvents({ addToURL: true, fromFilter: true });
	}

	async getEventDeliveries(requestDetails?: { addToURL?: boolean; fromFilter?: boolean }) {
		this.eventDeliveries && this.eventDeliveries?.pagination?.next === this.eventDeliveriesPage ? (this.isloadingMoreEventDeliveries = true) : (this.isloadingEventDeliveries = true);

		if (requestDetails?.addToURL) this.addFilterToURL({ section: 'eventDels' });
		const { startDate, endDate } = this.setDateForFilter(this.eventDeliveriesFilterDateRange.value);

		try {
			const eventDeliveriesResponse = await this.eventDeliveriesRequest({ eventId: this.eventDeliveryFilteredByEventId, startDate, endDate });

			if (this.eventDeliveries && this.eventDeliveries?.pagination?.next === this.eventDeliveriesPage) {
				const content = [...this.eventDeliveries.content, ...eventDeliveriesResponse.data.content];
				const pagination = eventDeliveriesResponse.data.pagination;
				this.eventDeliveries = { content, pagination };
				this.displayedEventDeliveries = this.setEventsDisplayed(content);
				this.isloadingMoreEventDeliveries = false;
				return;
			}

			this.eventDeliveries = eventDeliveriesResponse.data;
			this.displayedEventDeliveries = this.setEventsDisplayed(eventDeliveriesResponse.data.content);

			// if this is a filter request, set the eventDelsDetailsItem to the first item in the list
			if (requestDetails?.fromFilter) {
				this.eventDelsDetailsItem = this.eventDeliveries?.content[0];
				this.getDelieveryAttempts(this.eventDelsDetailsItem.uid);
			}

			this.isloadingEventDeliveries = false;
			return eventDeliveriesResponse.data.content;
		} catch (error) {
			this.isloadingEventDeliveries = false;
			this.isloadingMoreEventDeliveries = false;
			return error;
		}
	}

	async getEventDeliveriesForSidebar(eventId: string) {
		const response = await this.eventDeliveriesRequest({ eventId, startDate: '', endDate: '' });
		this.sidebarEventDeliveries = response.data.content;
	}

	async toggleActiveGroup() {
		await Promise.all([this.clearEventFilters('event deliveries'), this.clearEventFilters('events')]);
		this.addFilterToURL({ section: 'group' });
		Promise.all([this.getConfigDetails(), this.fetchDashboardData(), this.getEvents(), this.getApps({ type: 'apps' }), this.getEventDeliveries()]);
	}

	async getGroups(requestDetails?: { addToURL?: boolean }) {
		if (requestDetails?.addToURL) this.addFilterToURL({ section: 'group' });

		try {
			const groupsResponse = await this.convyDashboardService.request({
				url: this.getAPIURL(`/groups`),
				token: this.requestToken,
				authType: this.apiAuthType,
				method: 'get'
			});
			this.groups = groupsResponse.data;

			// check group existing filter in url and set active group
			if (!this.isCloud) this.activeGroup = this.route.snapshot.queryParams.group ?? this.groups[0]?.uid;
			return;
		} catch (error) {
			return error;
		}
	}

	async appsRequest(requestDetails: { search?: string }): Promise<HTTP_RESPONSE> {
		try {
			const appsResponse = await this.convyDashboardService.request({
				url: this.getAPIURL(`/apps?groupID=${this.activeGroup || ''}&sort=AESC&page=${this.appsPage || 1}&perPage=10${requestDetails?.search ? `&q=${requestDetails?.search}` : ''}`),
				token: this.requestToken,
				authType: this.apiAuthType,
				method: 'get'
			});

			return appsResponse;
		} catch (error: any) {
			return error;
		}
	}

	async getAppsForFilter(search: string): Promise<APP[]> {
		return await (
			await this.appsRequest({ search })
		).data.content;
	}

	async getApps(requestDetails?: { search?: string; type: 'filter' | 'apps' }) {
		if (this.apps?.pagination?.next === this.appsPage) this.isloadingMoreApps = true;
		if (requestDetails?.type === 'apps') this.isloadingApps = true;

		try {
			const appsResponse = await this.appsRequest({ search: requestDetails?.search });

			if (!requestDetails?.search && this.apps?.pagination?.next === this.appsPage) {
				const content = [...this.apps.content, ...appsResponse.data.content];
				const pagination = appsResponse.data.pagination;
				this.apps = { content, pagination };
				this.isloadingMoreApps = false;
				return;
			}

			if (requestDetails?.type === 'apps') this.apps = appsResponse.data;
			this.filteredApps = appsResponse.data.content;
			this.isloadingApps = false;
			return;
		} catch (error) {
			this.isloadingApps = false;
			this.isloadingMoreApps = false;
			return error;
		}
	}

	async getDelieveryAttempts(eventDeliveryId: string) {
		this.isloadingDeliveryAttempt = true;

		try {
			const deliveryAttemptsResponse = await this.convyDashboardService.request({
				url: this.getAPIURL(`/eventdeliveries/${eventDeliveryId}/deliveryattempts?groupID=${this.activeGroup || ''}`),
				token: this.requestToken,
				authType: this.apiAuthType,
				method: 'get'
			});
			this.eventDeliveryAtempt = deliveryAttemptsResponse.data[deliveryAttemptsResponse.data.length - 1];
			this.isloadingDeliveryAttempt = false;

			return;
		} catch (error) {
			this.isloadingDeliveryAttempt = false;
			return error;
		}
	}

	getCodeSnippetString(type: 'res_body' | 'event' | 'event_delivery' | 'res_head' | 'req' | 'error') {
		if (type === 'event') {
			if (!this.eventsDetailsItem?.data) return 'No event data was sent';
			return JSON.stringify(this.eventsDetailsItem?.data || this.eventsDetailsItem?.metadata?.data, null, 4).replaceAll(/"([^"]+)":/g, '$1:');
		} else if (type === 'event_delivery') {
			if (!this.eventDelsDetailsItem?.metadata?.data) return 'No event data was sent';
			return JSON.stringify(this.eventDelsDetailsItem.metadata.data, null, 4).replaceAll(/"([^"]+)":/g, '$1:');
		} else if (type === 'res_body') {
			if (!this.eventDeliveryAtempt) return 'No response body was sent';
			return this.eventDeliveryAtempt.response_data;
		} else if (type === 'res_head') {
			if (!this.eventDeliveryAtempt) return 'No response header was sent';
			return JSON.stringify(this.eventDeliveryAtempt.response_http_header, null, 4).replaceAll(/"([^"]+)":/g, '$1:');
		} else if (type === 'req') {
			if (!this.eventDeliveryAtempt) return 'No request header was sent';
			return JSON.stringify(this.eventDeliveryAtempt.request_http_header, null, 4).replaceAll(/"([^"]+)":/g, '$1:');
		} else if (type === 'error') {
			if (this.eventDeliveryAtempt?.error) return JSON.stringify(this.eventDeliveryAtempt.error, null, 4).replaceAll(/"([^"]+)":/g, '$1:');
			return '';
		}
		return '';
	}

	async retryEvent(requestDetails: { e: any; index: number; eventDeliveryId: string }) {
		requestDetails.e.stopPropagation();
		const retryButton: any = document.querySelector(`#event${requestDetails.index} button`);
		if (retryButton) {
			retryButton.classList.add(['spin', 'disabled']);
			retryButton.disabled = true;
		}

		try {
			await this.convyDashboardService.request({
				method: 'put',
				token: this.requestToken,
				authType: this.apiAuthType,
				url: this.getAPIURL(`/eventdeliveries/${requestDetails.eventDeliveryId}/resend?groupID=${this.activeGroup || ''}`)
			});

			this.convyDashboardService.showNotification({ message: 'Retry Request Sent' });
			retryButton.classList.remove(['spin', 'disabled']);
			retryButton.disabled = false;
			this.getEventDeliveries();
		} catch (error: any) {
			this.convyDashboardService.showNotification({ message: error.error.message });
			retryButton.classList.remove(['spin', 'disabled']);
			retryButton.disabled = false;
			return error;
		}
	}

	async batchRetryEvent() {
		try {
			await this.convyDashboardService.request({
				method: 'post',
				url: this.getAPIURL(`/eventdeliveries/batchretry?groupID=${this.activeGroup || ''}`),
				token: this.requestToken,
				authType: this.apiAuthType,
				body: { ids: this.selectedEventsFromEventDeliveriesTable }
			});

			this.convyDashboardService.showNotification({ message: 'Batch Retry Request Sent' });
			this.getEventDeliveries();
			this.selectedEventsFromEventDeliveriesTable = [];
		} catch (error: any) {
			this.convyDashboardService.showNotification({ message: error.error.message });
			return error;
		}
	}

	async clearEventFilters(tableName: 'events' | 'event deliveries' | 'apps') {
		const activeFilters = Object.assign({}, this.route.snapshot.queryParams);
		let filterItems: string[] = [];

		switch (tableName) {
			case 'events':
				this.eventApp = '';
				filterItems = ['eventsStartDate', 'eventsEndDate', 'eventsApp'];
				this.eventsFilterDateRange.patchValue({ startDate: '', endDate: '' });
				this.getEvents({ fromFilter: true });
				break;

			case 'event deliveries':
				this.eventDeliveriesApp = '';
				filterItems = ['eventDelsStartDate', 'eventDelsEndDate', 'eventDelsApp', 'eventDelsStatus'];
				this.eventDeliveriesFilterDateRange.patchValue({ startDate: '', endDate: '' });
				this.eventDeliveryFilteredByEventId = '';
				this.eventDeliveryFilteredByStatus = [];
				this.getEventDeliveries({ fromFilter: true });
				break;

			default:
				break;
		}

		filterItems.forEach(key => (activeFilters.hasOwnProperty(key) ? delete activeFilters[key] : null));
		await this.router.navigate([], { relativeTo: this.route, queryParams: activeFilters });
	}

	checkAllCheckboxes(event: any) {
		const checkboxes = document.querySelectorAll('#event-deliveries-table tbody input[type="checkbox"]');

		checkboxes.forEach((checkbox: any) => {
			this.selectedEventsFromEventDeliveriesTable.push(checkbox.value);
			checkbox.checked = event.target.checked;
		});

		if (!event.target.checked) this.selectedEventsFromEventDeliveriesTable = [];
		this.allEventdeliveriesChecked = event.target.checked;
	}

	checkEventDeliveryBox(event: any) {
		const checkbox = event.target;
		if (checkbox.checked) {
			this.selectedEventsFromEventDeliveriesTable.push(checkbox.value);
		} else {
			this.selectedEventsFromEventDeliveriesTable = this.selectedEventsFromEventDeliveriesTable.filter(eventId => eventId !== checkbox.value);
		}
		this.allEventdeliveriesChecked = false;
		const parentCheckbox: any = document.querySelector('#eventDeliveryTable');
		parentCheckbox.checked = false;
	}

	async loadMoreEventDeliveries() {
		this.eventDeliveriesPage = this.eventDeliveriesPage + 1;
		await this.getEventDeliveries();
		setTimeout(() => {
			if (this.allEventdeliveriesChecked) {
				this.checkAllCheckboxes({ target: { checked: true } });
			}
		}, 200);
	}

	async openDeliveriesTab() {
		await this.getEventDeliveries({ addToURL: true });
		delete this.eventDelsDetailsItem;
		this.toggleActiveTab('event deliveries');
	}

	async refreshTables() {
		await this.initDashboard();
		this.toggleActiveTab('event deliveries');
	}

	getAPIURL(url: string) {
		return this.apiURL + url;
	}

	checkIfEventDeliveryStatusFilterOptionIsSelected(status: string): boolean {
		return this.eventDeliveryFilteredByStatus?.length > 0 ? this.eventDeliveryFilteredByStatus.includes(status) : false;
	}

	checkIfEventDeliveryAppFilterOptionIsSelected(appId: string): boolean {
		return appId === this.eventDeliveriesApp;
	}

	searchApps(searchDetails: { searchInput?: any; type: 'filter' | 'apps' }) {
		const searchString: string = searchDetails?.searchInput?.target?.value || this.appsSearchString;
		if (searchString) {
			this.getApps({ search: searchString, type: searchDetails.type });
		} else {
			searchDetails.type === 'filter' ? (this.filteredApps = this.apps.content) : this.getApps({ type: 'apps' });
		}
	}
}
