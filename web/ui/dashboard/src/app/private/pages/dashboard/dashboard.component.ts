import { Component, OnInit } from '@angular/core';
import { HttpService } from 'src/app/services/http/http.service';
import Chart from 'chart.js/auto';
import * as moment from 'moment';
import { GeneralService } from 'src/app/services/general/general.service';
import { APP } from 'src/app/models/app.model';
import { EVENT, EVENT_DELIVERY } from 'src/app/models/event.model';
import { ActivatedRoute, Router } from '@angular/router';
import { FormBuilder, FormGroup } from '@angular/forms';
import { PAGINATION } from 'src/app/models/global.model';
import { HTTP_RESPONSE } from 'src/app/models/http.model';
import { GROUP } from 'src/app/models/group.model';

@Component({
	selector: 'app-dashboard',
	templateUrl: './dashboard.component.html',
	styleUrls: ['./dashboard.component.scss']
})
export class DashboardComponent implements OnInit {
	showDropdown = false;
	showFilterCalendar = false;
	tabs: ['events', 'event deliveries', 'apps'] = ['events', 'event deliveries', 'apps'];
	activeTab: 'events' | 'apps' | 'event deliveries' = 'events';
	detailsItem?: any;
	eventDeliveryAtempt?: {
		ip_address: '';
		http_status: '';
		api_version: '';
		updated_at: 0;
		deleted_at: 0;
		response_data: '';
		response_http_header: '';
		request_http_header: '';
	};
	showEventFilterCalendar = false;
	eventDateFilterActive = false;
	displayedEvents: {
		date: string;
		events: EVENT[];
	}[] = [];
	events!: { pagination: PAGINATION; content: EVENT[] };
	apps!: { pagination: PAGINATION; content: APP[] };
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
	eventDeliveryStatuses = ['', 'Success', 'Failure', 'Retry', 'Scheduled', 'Processing', 'Discarded'];
	eventDeliveryFilteredByStatus: '' | 'Success' | 'Failure' | 'Retry' | 'Scheduled' | 'Processing' | 'Discarded' = '';
	showOverlay = false;
	showEventDeliveriesStatusDropdown = false;

	constructor(private httpService: HttpService, private generalService: GeneralService, private router: Router, private formBuilder: FormBuilder, private route: ActivatedRoute) {}

	async ngOnInit() {
		await this.initDashboard();
	}

	async initDashboard() {
		await this.getGroups();
		this.getFiltersFromURL();
		await Promise.all([this.getConfigDetails(), this.fetchDashboardData(), this.getEvents(), this.getApps(), this.getEventDeliveries()]);

		// get active tab from url and apply, after getting the details from above requests so that the data is available ahead
		this.toggleActiveTab(this.route.snapshot.queryParams.activeTab ?? 'events');
		return;
	}

	logout() {
		localStorage.removeItem('CONVOY_AUTH');
		this.router.navigateByUrl('/login');
	}

	toggleActiveTab(tab: 'events' | 'apps' | 'event deliveries') {
		this.activeTab = tab;
		this.addFilterToURL({ section: 'logTab' });

		if (tab === 'apps' && this.apps?.content.length > 0) {
			this.detailsItem = this.apps?.content[0];
		} else if (tab === 'events' && this.events?.content.length > 0) {
			this.eventDetailsActiveTab = 'data';
			this.detailsItem = this.events?.content[0];
			this.getEventDeliveriesForSidebar(this.detailsItem.uid);
		} else if (tab === 'event deliveries' && this.eventDeliveries?.content.length > 0) {
			this.detailsItem = this.eventDeliveries?.content[0];
			this.getDelieveryAttempts(this.detailsItem.uid);
		}
	}

	async getConfigDetails() {
		try {
			const organisationDetailsResponse = await this.httpService.request({
				url: `/dashboard/config?groupID=${this.activeGroup || ''}`,
				method: 'get'
			});
			this.organisationDetails = organisationDetailsResponse.data;
		} catch (error) {}
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
		this.eventDeliveryFilteredByStatus = filters.eventDelsStatus ?? null;
	}

	async fetchDashboardData() {
		try {
			const { startDate, endDate } = this.setDateForFilter(this.statsDateRange.value);

			const dashboardResponse = await this.httpService.request({
				url: `/dashboard/summary?groupID=${this.activeGroup || ''}&startDate=${startDate || ''}&endDate=${endDate || ''}&type=${this.dashboardFrequency}`,
				method: 'get'
			});
			this.dashboardData = dashboardResponse.data;

			let labelsDateFormat = '';
			if (this.dashboardFrequency === 'daily') labelsDateFormat = 'DD[, ]MMM';
			else if (this.dashboardFrequency === 'monthly') labelsDateFormat = 'MMM';
			else if (this.dashboardFrequency === 'yearly') labelsDateFormat = 'YYYY';

			const chartData = dashboardResponse.data.event_data;
			const labels = [...chartData.map((label: { data: { date: any } }) => label.data.date)].map(date => (this.dashboardFrequency === 'weekly' ? date : moment(date).format(labelsDateFormat)));
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

			if (!Chart.getChart('chart') || !Chart.getChart('chart')?.canvas) {
				new Chart('chart', { type: 'line', data, options });
			} else {
				const currentChart = Chart.getChart('chart');
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
		const startDate = requestDetails.startDate ? String(moment(`${moment(requestDetails.startDate.toISOString()).format('YYYY[-]MM[-]DD')} 00:00:00`).toISOString(true)).split('.')[0] : '';
		const endDate = requestDetails.endDate ? String(moment(`${moment(requestDetails.endDate.toISOString()).format('YYYY[-]MM[-]DD')} 23:59:59`).toISOString(true)).split('.')[0] : '';
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

	async getEvents(requestDetails?: { appId?: string; addToURL?: boolean }) {
		if (requestDetails?.appId) this.eventApp = requestDetails.appId;
		if (requestDetails?.addToURL) this.addFilterToURL({ section: 'events' });

		const { startDate, endDate } = this.setDateForFilter(this.eventsFilterDateRange.value);

		try {
			const eventsResponse = await this.httpService.request({
				url: `/events?groupID=${this.activeGroup || ''}&sort=AESC&page=${this.eventsPage || 1}&perPage=20&startDate=${startDate}&endDate=${endDate}&appId=${requestDetails?.appId ?? this.eventApp}`,
				method: 'get'
			});
			if (this.activeTab === 'events') this.detailsItem = eventsResponse.data.content[0];

			if (this.events && this.events?.pagination?.next === this.eventsPage) {
				const content = [...this.events.content, ...eventsResponse.data.content];
				const pagination = eventsResponse.data.pagination;
				this.events = { content, pagination };
				this.displayedEvents = this.setEventsDisplayed(content);
				return;
			}

			this.events = eventsResponse.data;
			this.displayedEvents = await this.setEventsDisplayed(eventsResponse.data.content);
		} catch (error) {
			return error;
		}
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
			queryParams.eventDelsStatus = this.eventDeliveryFilteredByStatus || '';
		}

		if (requestDetails.section === 'group') queryParams.group = this.activeGroup;

		if (requestDetails.section === 'logTab') queryParams.activeTab = this.activeTab;

		this.router.navigate([], { queryParams: Object.assign({}, currentURLfilters, queryParams) });
	}

	async eventDeliveriesRequest(requestDetails: { eventId?: string; startDate?: string; endDate?: string }): Promise<HTTP_RESPONSE> {
		const { startDate, endDate } = this.setDateForFilter(this.eventDeliveriesFilterDateRange.value);

		try {
			const eventDeliveriesResponse = await this.httpService.request({
				url: `/eventdeliveries?groupID=${this.activeGroup || ''}&eventId=${requestDetails.eventId || ''}&page=${this.eventDeliveriesPage || 1}&startDate=${startDate}&endDate=${endDate}&appId=${
					this.eventDeliveriesApp
				}&status=${this.eventDeliveryFilteredByStatus || ''}`,
				method: 'get'
			});

			return eventDeliveriesResponse;
		} catch (error: any) {
			return error;
		}
	}

	async getEventDeliveries(requestDetails?: { addToURL?: boolean }) {
		if (requestDetails?.addToURL) this.addFilterToURL({ section: 'eventDels' });
		const { startDate, endDate } = this.setDateForFilter(this.eventDeliveriesFilterDateRange.value);

		try {
			const eventDeliveriesResponse = await this.eventDeliveriesRequest({ eventId: this.eventDeliveryFilteredByEventId, startDate, endDate });
			if (this.activeTab === 'event deliveries') this.detailsItem = eventDeliveriesResponse.data.content[0];

			if (this.eventDeliveries && this.eventDeliveries?.pagination?.next === this.eventDeliveriesPage) {
				const content = [...this.eventDeliveries.content, ...eventDeliveriesResponse.data.content];
				const pagination = eventDeliveriesResponse.data.pagination;
				this.eventDeliveries = { content, pagination };
				this.displayedEventDeliveries = this.setEventsDisplayed(content);
				return;
			}

			this.eventDeliveries = eventDeliveriesResponse.data;
			this.displayedEventDeliveries = this.setEventsDisplayed(eventDeliveriesResponse.data.content);
			return eventDeliveriesResponse.data.content;
		} catch (error) {
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
		Promise.all([this.getConfigDetails(), this.fetchDashboardData(), this.getEvents(), this.getApps(), this.getEventDeliveries()]);
	}

	async getGroups(requestDetails?: { addToURL?: boolean }) {
		if (requestDetails?.addToURL) this.addFilterToURL({ section: 'group' });

		try {
			const groupsResponse = await this.httpService.request({
				url: `/groups`,
				method: 'get'
			});
			this.groups = groupsResponse.data;

			// check group existing filter in url and set active group
			this.activeGroup = this.route.snapshot.queryParams.group ?? this.groups[0]?.uid;
			return;
		} catch (error) {
			return error;
		}
	}

	async getApps() {
		try {
			const appsResponse = await this.httpService.request({
				url: `/apps?groupID=${this.activeGroup || ''}&sort=AESC&page=${this.appsPage || 1}&perPage=10`,
				method: 'get'
			});

			if (this.apps?.pagination?.next === this.appsPage) {
				const content = [...this.apps.content, ...appsResponse.data.content];
				const pagination = appsResponse.data.pagination;
				this.apps = { content, pagination };
				return;
			}
			this.apps = appsResponse.data;
			if (this.activeTab === 'apps') this.detailsItem = this.apps?.content[0];
			return;
		} catch (error) {
			return error;
		}
	}

	async getDelieveryAttempts(eventDeliveryId: string) {
		try {
			const deliveryAttemptsResponse = await this.httpService.request({
				url: `/eventdeliveries/${eventDeliveryId}/deliveryattempts?groupID=${this.activeGroup || ''}`,
				method: 'get'
			});
			this.eventDeliveryAtempt = deliveryAttemptsResponse.data[deliveryAttemptsResponse.data.length - 1];
			return;
		} catch (error) {
			return error;
		}
	}

	getCodeSnippetString(type: 'res_body' | 'event' | 'event_delivery' | 'res_head' | 'req') {
		if (type === 'event') {
			if (!this.detailsItem?.data) return 'No event data was sent';
			return JSON.stringify(this.detailsItem.data || this.detailsItem.metadata.data, null, 4).replaceAll(/"([^"]+)":/g, '$1:');
		} else if (type === 'event_delivery') {
			if (!this.detailsItem?.metadata?.data) return 'No event data was sent';
			return JSON.stringify(this.detailsItem.metadata.data, null, 4).replaceAll(/"([^"]+)":/g, '$1:');
		} else if (type === 'res_body') {
			if (!this.eventDeliveryAtempt) return 'No response body was sent';
			return this.eventDeliveryAtempt.response_data;
		} else if (type === 'res_head') {
			if (!this.eventDeliveryAtempt) return 'No response header was sent';
			return JSON.stringify(this.eventDeliveryAtempt.response_http_header, null, 4).replaceAll(/"([^"]+)":/g, '$1:');
		} else if (type === 'req') {
			if (!this.eventDeliveryAtempt) return 'No request header was sent';
			return JSON.stringify(this.eventDeliveryAtempt.request_http_header, null, 4).replaceAll(/"([^"]+)":/g, '$1:');
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
			await this.httpService.request({
				method: 'put',
				url: `/eventdeliveries/${requestDetails.eventDeliveryId}/resend?groupID=${this.activeGroup || ''}`
			});

			this.generalService.showNotification({ message: 'Retry Request Sent' });
			retryButton.classList.remove(['spin', 'disabled']);
			retryButton.disabled = false;
			this.getEventDeliveries();
		} catch (error: any) {
			this.generalService.showNotification({ message: error.error.message });
			retryButton.classList.remove(['spin', 'disabled']);
			retryButton.disabled = false;
			return error;
		}
	}

	async batchRetryEvent() {
		try {
			await this.httpService.request({
				method: 'post',
				url: `/eventdeliveries/batchretry?groupID=${this.activeGroup || ''}`,
				body: { ids: this.selectedEventsFromEventDeliveriesTable }
			});

			this.generalService.showNotification({ message: 'Batch Retry Request Sent' });
			this.getEventDeliveries();
			this.selectedEventsFromEventDeliveriesTable = [];
		} catch (error: any) {
			this.generalService.showNotification({ message: error.error.message });
			return error;
		}
	}

	authDetails() {
		const authDetails = localStorage.getItem('CONVOY_AUTH');
		return authDetails ? JSON.parse(authDetails) : false;
	}

	async clearEventFilters(tableName: 'events' | 'event deliveries') {
		const activeFilters = Object.assign({}, this.route.snapshot.queryParams);
		let filterItems: string[] = [];

		switch (tableName) {
			case 'events':
				this.eventApp = '';
				filterItems = ['eventsStartDate', 'eventsEndDate', 'eventsApp'];
				this.eventsFilterDateRange.patchValue({ startDate: '', endDate: '' });
				this.getEvents();
				break;

			case 'event deliveries':
				this.eventDeliveriesApp = '';
				filterItems = ['eventDelsStartDate', 'eventDelsEndDate', 'eventDelsApp', 'eventDelsStatus'];
				this.eventDeliveriesFilterDateRange.patchValue({ startDate: '', endDate: '' });
				this.eventDeliveryFilteredByEventId = '';
				this.eventDeliveryFilteredByStatus = '';
				this.getEventDeliveries();
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
		await this.getEventDeliveries();
		this.toggleActiveTab('event deliveries');
	}

	async refreshTables() {
		await this.initDashboard();
		this.toggleActiveTab('event deliveries');
	}
}
