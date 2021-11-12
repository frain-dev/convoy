import { Component, OnInit } from '@angular/core';
import { HttpService } from 'src/app/services/http/http.service';
import Chart from 'chart.js/auto';
import * as moment from 'moment';
import { GeneralService } from 'src/app/services/general/general.service';
import { APP } from 'src/app/models/app.model';
import { EVENT, EVENT_DELIVERY } from 'src/app/models/event.model';
import { Router } from '@angular/router';
import { FormBuilder, FormGroup } from '@angular/forms';

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
	events!: { pagination: { next: number; page: number; perPage: number; prev: number; total: number; totalPage: number }; content: EVENT[] };
	apps!: { pagination: { next: number; page: number; perPage: number; prev: number; total: number; totalPage: number }; content: APP[] };
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
	eventsPage: number = 1;
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
	selectedEventsFromEventsTable: string[] = [];
	displayedEventDeliveries: {
		date: string;
		events: EVENT_DELIVERY[];
	}[] = [];
	eventDeliveries: EVENT_DELIVERY[] = [];

	constructor(private httpService: HttpService, private generalService: GeneralService, private router: Router, private formBuilder: FormBuilder) {}

	async ngOnInit() {
		await this.initDashboard();
		this.toggleActiveTab('events');
	}

	async initDashboard() {
		await Promise.all([this.getOrganisationDetails(), this.fetchDashboardData(), this.getEvents(), this.getApps(), this.getEventDeliveries()]);
	}

	toggleShowDropdown() {}

	logout() {
		localStorage.removeItem('CONVOY_AUTH');
		this.router.navigateByUrl('/login');
	}

	toggleActiveTab(tab: 'events' | 'apps' | 'event deliveries') {
		this.activeTab = tab;

		if (tab === 'apps' && this.apps?.content.length > 0) {
			this.detailsItem = this.apps?.content[0];
		} else if (tab === 'events' && this.events?.content.length > 0) {
			this.eventDetailsActiveTab = 'data';
			this.detailsItem = this.events?.content[0];
		} else if (tab === 'event deliveries' && this.eventDeliveries?.length > 0) {
			this.detailsItem = this.eventDeliveries?.[0];
			this.getDelieveryAttempts({ eventDeliveryId: this.detailsItem.uid, eventId: this.detailsItem.event_id });
		}
	}

	async getOrganisationDetails() {
		try {
			const organisationDetailsResponse = await this.httpService.request({
				url: `/dashboard/config`,
				method: 'get'
			});
			this.organisationDetails = organisationDetailsResponse.data;
		} catch (error) {}
	}

	async fetchDashboardData() {
		try {
			const { startDate, endDate } = this.setDateForFilter(this.statsDateRange.value);
			const dashboardResponse = await this.httpService.request({
				url: `/dashboard/summary?startDate=${startDate || ''}&endDate=${endDate || ''}&type=${this.dashboardFrequency}`,
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

	getTime(date: Date) {
		const _date = new Date(date);
		const hours = _date.getHours();
		const minutes = _date.getMinutes();
		const seconds = _date.getSeconds();

		const hour = hours > 12 ? hours - 12 : hours;
		return `${hour} : ${minutes > 9 ? minutes : '0' + minutes} : ${seconds > 9 ? seconds : '0' + seconds} ${hours > 12 ? 'AM' : 'PM'}`;
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

	async getEvents() {
		const { startDate, endDate } = this.setDateForFilter(this.eventsFilterDateRange.value);

		try {
			const eventsResponse = await this.httpService.request({
				url: `/events?sort=AESC&page=${this.eventsPage || 1}&perPage=20&startDate=${startDate}&endDate=${endDate}&appId=${this.eventApp}`,
				method: 'get'
			});
			this.detailsItem = eventsResponse.data.content[0];

			if (this.events && this.events?.pagination?.next === this.eventsPage) {
				const content = [...this.events.content, ...eventsResponse.data.content];
				const pagination = eventsResponse.data.pagination;
				this.events = { content, pagination };
				this.setEventsDisplayed(content);
				return;
			}

			this.events = eventsResponse.data;
			this.displayedEvents = await this.setEventsDisplayed(eventsResponse.data.content);
		} catch (error) {
			return error;
		}
	}

	async getEventDeliveries(eventId?: string) {
		const { startDate, endDate } = this.setDateForFilter(this.eventsFilterDateRange.value);

		try {
			const eventDeliveriesResponse = await this.httpService.request({
				url: `/events/${eventId || 'a6cf38e4-f8fd-433c-ac5a-45c8b5f410b1'}/eventdeliveries`,
				method: 'get'
			});
			console.log('🚀 ~ file: dashboard.component.ts ~ line 238 ~ DashboardComponent ~ getEventsDeliveries ~ eventsResponse', eventDeliveriesResponse);

			// if (this.events && this.events?.pagination?.next === this.eventsPage) {
			// 	const content = [...this.events.content, ...eventsResponse.data.content];
			// 	const pagination = eventsResponse.data.pagination;
			// 	this.events = { content, pagination };
			// 	this.setEventsDisplayed(content);
			// 	return;
			// }

			this.eventDeliveries = eventDeliveriesResponse.data;
			this.getDelieveryAttempts({ eventDeliveryId: this.eventDeliveries[0].uid, eventId: this.eventDeliveries[0].event_id });
			this.displayedEventDeliveries = this.setEventsDisplayed(eventDeliveriesResponse.data);
		} catch (error) {
			return error;
		}
	}

	async getApps() {
		try {
			const appsResponse = await this.httpService.request({
				url: `/apps?sort=AESC&page=${this.appsPage || 1}&perPage=10`,
				method: 'get'
			});

			if (this.apps?.pagination?.next === this.appsPage) {
				const content = [...this.apps.content, ...appsResponse.data.content];
				const pagination = appsResponse.data.pagination;
				this.apps = { content, pagination };
				return;
			}
			this.apps = appsResponse.data;
		} catch (error) {
			return error;
		}
	}

	async getDelieveryAttempts(requestDetails: { eventId: string; eventDeliveryId: string }) {
		try {
			const deliveryAttemptsResponse = await this.httpService.request({
				url: `/events/${requestDetails.eventId}/eventdeliveries/${requestDetails.eventDeliveryId}/deliveryattempts`,
				method: 'get'
			});
			this.eventDeliveryAtempt = deliveryAttemptsResponse.data[deliveryAttemptsResponse.data.length - 1];
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

	async retryEvent(requestDetails: { eventId: string; appId: string; e: any; index: number; eventDeliveryId: string }) {
		requestDetails.e.stopPropagation();
		const retryButton: any = document.querySelector(`#event${requestDetails.index} button`);
		if (retryButton) {
			retryButton.classList.add(['spin', 'disabled']);
			retryButton.disabled = true;
		}

		try {
			await this.httpService.request({
				method: 'put',
				url: `/apps/${requestDetails.appId}/events/${requestDetails.eventId}/eventdeliveries/${requestDetails.eventDeliveryId}/resend`
			});
			this.generalService.showNotification({ message: 'Retry Request Sent' });
			retryButton.classList.remove(['spin', 'disabled']);
			retryButton.disabled = false;
			this.getEvents();
		} catch (error: any) {
			this.generalService.showNotification({ message: error.error.message });
			retryButton.classList.remove(['spin', 'disabled']);
			retryButton.disabled = false;
			return error;
		}
	}

	authDetails() {
		const authDetails = localStorage.getItem('CONVOY_AUTH');
		return authDetails ? JSON.parse(authDetails) : false;
	}

	clearEventFilters() {
		this.eventApp = '';
		this.eventsFilterDateRange.patchValue({ startDate: '', endDate: '' });
		this.getEvents();
	}

	checkAllCheckboxes(event: any) {
		const checkboxes = document.querySelectorAll('#events-table tbody input[type="checkbox"]');
		checkboxes.forEach((checkbox: any) => {
			this.selectedEventsFromEventsTable.push(checkbox.value);
			checkbox.checked = event.target.checked;
		});
	}

	async openDeliveriesTab(eventId: string) {
		await this.getEventDeliveries(eventId);
		this.toggleActiveTab('event deliveries');
	}
}
