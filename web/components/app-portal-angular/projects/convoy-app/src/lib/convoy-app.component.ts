import { Component, Input, OnInit } from '@angular/core';
import { APP } from './models/app.model';
import { EVENT, EVENT_DELIVERY } from './models/event.model';
import { ActivatedRoute, Router } from '@angular/router';
import { FormBuilder, FormControl, FormGroup, Validators } from '@angular/forms';
import { PAGINATION } from './models/global.model';
import { HTTP_RESPONSE } from './models/http.model';
import { ConvoyAppService } from './convoy-app.service';
import { format } from 'date-fns';
import { DatePipe } from '@angular/common';

@Component({
	selector: 'convoy-app',
	templateUrl: './convoy-app.component.html',
	styleUrls: ['./convoy-app.component.scss']
})
export class ConvoyAppComponent implements OnInit {
	addNewEndpointForm: FormGroup = this.formBuilder.group({
		url: ['', Validators.required],
		events: [''],
		description: ['', Validators.required]
	});
	eventTags: string[] = [];
	tabs: ['events', 'event deliveries'] = ['events', 'event deliveries'];
	activeTab: 'events' | 'event deliveries' = 'events';
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
	appDetails!: APP;
	eventDetailsTabs = [
		{ id: 'data', label: 'Event' },
		{ id: 'response', label: 'Response' },
		{ id: 'request', label: 'Request' }
	];
	selectedEventsDateOption = '';
	selectedEventsDelDateOption = '';
	selectedEventsDelTimeOption = '';
	selectedEventsTimeOption = '';
	timeFilter!: any;
	eventDetailsActiveTab = 'data';
	eventApp: string = '';
	eventsPage: number = 1;
	eventDeliveriesPage: number = 1;
	appsPage: number = 1;
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
	batchRetryCount!: any;
	allEventdeliveriesChecked = false;
	dateOptions = ['Last Year', 'Last Month', 'Last Week', 'Yesterday'];
	eventDeliveryStatuses = ['Success', 'Failure', 'Retry', 'Scheduled', 'Processing', 'Discarded'];
	eventDeliveryFilteredByStatus: string[] = [];
	showTimePicker = false;
	showOverlay = false;
	showEventDeliveriesStatusDropdown = false;
	isRetyring = false;
	isloadingMoreEvents = false;
	isloadingEvents = false;
	showBatchRetryModal = false;
	fetchingCount = false;
	showAddEndpointModal = false;
	isCreatingNewEndpoint = false;
	@Input('token') token!: string;
	@Input('apiURL') apiURL: string = '';

	constructor(private convyAppService: ConvoyAppService, private router: Router, private formBuilder: FormBuilder, private route: ActivatedRoute, private datePipe: DatePipe) {}

	async ngOnInit() {
		await this.initDashboard();
	}

	async initDashboard() {
		await Promise.all([this.getEvents(), this.getEventDeliveries(), this.getAppDetails()]);

		// get active tab from url and apply, after getting the details from above requests so that the data is available ahead
		this.toggleActiveTab(this.route.snapshot.queryParams.activeTab ?? 'events');
		return;
	}

	toggleActiveTab(tab: 'events' | 'event deliveries') {
		this.activeTab = tab;

		if (tab === 'events' && this.events?.content.length > 0) {
			this.eventDetailsActiveTab = 'data';
			this.detailsItem = this.events?.content[0];
			this.getEventDeliveriesForSidebar(this.detailsItem.uid);
		} else if (tab === 'event deliveries' && this.eventDeliveries?.content.length > 0) {
			this.detailsItem = this.eventDeliveries?.content[0];
			this.getDelieveryAttempts(this.detailsItem.uid);
		}
	}

	setDateForFilter(requestDetails: { startDate: Date; endDate: Date }) {
		if (!requestDetails.endDate && !requestDetails.startDate) return { startDate: '', endDate: '' };
		const startDate = requestDetails.startDate ? `${format(requestDetails.startDate, 'yyyy-MM-dd')}T00:00:00` : '';
		const endDate = requestDetails.endDate ? `${format(requestDetails.endDate, 'yyyy-MM-dd')}T23:59:59` : '';
		return { startDate, endDate };
	}

	getSelectedDate(dateOption: string, activeTab: string) {
		activeTab == 'events' ? (this.selectedEventsDateOption = dateOption) : (this.selectedEventsDelDateOption = dateOption);
		const _date = new Date();
		let startDate, endDate, currentDayOfTheWeek;
		switch (dateOption) {
			case 'Last Year':
				startDate = new Date(_date.getFullYear() - 1, 0, 1);
				endDate = new Date(_date.getFullYear(), _date.getMonth(), _date.getDate());
				break;
			case 'Last Month':
				startDate = new Date(_date.getFullYear(), _date.getMonth() == 0 ? 11 : _date.getMonth() - 1, 1);
				endDate = new Date(_date.getFullYear(), _date.getMonth(), _date.getDate());
				break;
			case 'Last Week':
				currentDayOfTheWeek = _date.getDay();
				switch (currentDayOfTheWeek) {
					case 0:
						startDate = new Date(_date.getFullYear(), _date.getMonth(), _date.getDate() - 7);
						endDate = new Date(_date.getFullYear(), _date.getMonth(), _date.getDate());
						break;
					case 1:
						startDate = new Date(_date.getFullYear(), _date.getMonth(), _date.getDate() - 8);
						endDate = new Date(_date.getFullYear(), _date.getMonth(), _date.getDate());
						break;
					case 2:
						startDate = new Date(_date.getFullYear(), _date.getMonth(), _date.getDate() - 9);
						endDate = new Date(_date.getFullYear(), _date.getMonth(), _date.getDate());
						break;
					case 3:
						startDate = new Date(_date.getFullYear(), _date.getMonth(), _date.getDate() - 10);
						endDate = new Date(_date.getFullYear(), _date.getMonth(), _date.getDate());
						break;
					case 4:
						startDate = new Date(_date.getFullYear(), _date.getMonth(), _date.getDate() - 11);
						endDate = new Date(_date.getFullYear(), _date.getMonth(), _date.getDate());
						break;
					case 4:
						startDate = new Date(_date.getFullYear(), _date.getMonth(), _date.getDate() - 12);
						endDate = new Date(_date.getFullYear(), _date.getMonth(), _date.getDate());
						break;
					case 5:
						startDate = new Date(_date.getFullYear(), _date.getMonth(), _date.getDate() - 13);
						endDate = new Date(_date.getFullYear(), _date.getMonth(), _date.getDate());
						break;
					case 6:
						startDate = new Date(_date.getFullYear(), _date.getMonth(), _date.getDate() - 14);
						endDate = new Date(_date.getFullYear(), _date.getMonth(), _date.getDate());
						break;
					default:
						break;
				}
				break;
			case 'Yesterday':
				startDate = new Date(_date.getFullYear(), _date.getMonth(), _date.getDate() - 1);
				endDate = new Date(_date.getFullYear(), _date.getMonth(), _date.getDate());
				break;
			default:
				break;
		}

		if (activeTab == 'events') {
			this.eventsFilterDateRange.patchValue({
				startDate: startDate,
				endDate: endDate
			});
		} else {
			this.eventDeliveriesFilterDateRange.patchValue({
				startDate: startDate,
				endDate: endDate
			});
		}

		activeTab == 'events' ? this.getEvents() : this.getEventDeliveries();
	}

	getDate(date: Date) {
		const months = ['Jan', 'Feb', 'Mar', 'April', 'May', 'June', 'July', 'Aug', 'Sept', 'Oct', 'Nov', 'Dec'];
		const _date = new Date(date);
		const day = _date.getDate();
		const month = _date.getMonth();
		const year = _date.getFullYear();
		return `${day} ${months[month]}, ${year}`;
	}

	getSelectedTime(e: any, activeTab: string) {
		const timePicked = e.target.value;
		activeTab == 'events' ? (this.selectedEventsTimeOption = timePicked) : (this.selectedEventsDelTimeOption = timePicked);
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

	async getEvents(requestDetails?: { appId?: string }) {
		this.events?.pagination?.next === this.eventsPage ? (this.isloadingMoreEvents = true) : (this.isloadingEvents = true);
		if (requestDetails?.appId) this.eventApp = requestDetails.appId;

		const { startDate, endDate } = this.setDateForFilter(this.eventsFilterDateRange.value);

		try {
			const eventsResponse = await this.convyAppService.request({
				url: this.getAPIURL(`/events?sort=AESC&page=${this.eventsPage || 1}&startDate=${startDate}&endDate=${endDate}`),
				method: 'get',
				token: this.token
			});
			if (this.activeTab === 'events') this.detailsItem = eventsResponse.data.content[0];

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
			this.isloadingEvents = false;
		} catch (error) {
			this.isloadingEvents = false;
			this.isloadingMoreEvents = false;
			return error;
		}
	}

	async getAppDetails() {
		try {
			const appDetailsResponse = await this.convyAppService.request({
				url: this.getAPIURL(`/apps`),
				method: 'get',
				token: this.token
			});

			this.appDetails = appDetailsResponse.data;
		} catch (error) {
			return error;
		}
	}

	async eventDeliveriesRequest(requestDetails: { eventId?: string; startDate?: string; endDate?: string }): Promise<HTTP_RESPONSE> {
		let eventDeliveryStatusFilterQuery = '';
		this.eventDeliveryFilteredByStatus.forEach((status: string) => (eventDeliveryStatusFilterQuery += `&status=${status}`));
		const { startDate, endDate } = this.setDateForFilter(this.eventDeliveriesFilterDateRange.value);

		try {
			const eventDeliveriesResponse = await this.convyAppService.request({
				url: this.getAPIURL(
					`/eventdeliveries?eventId=${requestDetails.eventId || ''}&page=${this.eventDeliveriesPage || 1}&startDate=${startDate}&endDate=${endDate}&status=${eventDeliveryStatusFilterQuery || ''}`
				),
				method: 'get',
				token: this.token
			});

			return eventDeliveriesResponse;
		} catch (error: any) {
			return error;
		}
	}

	async getEventDeliveries() {
		const { startDate, endDate } = this.setDateForFilter(this.eventDeliveriesFilterDateRange.value);

		try {
			const eventDeliveriesResponse = await this.eventDeliveriesRequest({
				eventId: this.eventDeliveryFilteredByEventId,
				startDate,
				endDate
			});
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

	// add new endpoint to app
	async addNewEndpoint(appUid?: string) {
		if (this.addNewEndpointForm.invalid) {
			(<any>Object).values(this.addNewEndpointForm.controls).forEach((control: FormControl) => {
				control?.markAsTouched();
			});
			return;
		}
		this.isCreatingNewEndpoint = true;

		this.addNewEndpointForm.patchValue({
			events: this.eventTags
		});
		try {
			const response = await this.convyAppService.request({
				url: this.getAPIURL(
					`/apps/endpoints`
				),
				method: 'post',
				body: this.addNewEndpointForm.value,
				token: this.token
			});
			this.convyAppService.showNotification({ message: response.message });
			this.getAppDetails();
			this.getEvents();
			this.addNewEndpointForm.reset();
			this.eventTags = [];
			this.showAddEndpointModal = false;
			this.isCreatingNewEndpoint = false;
		} catch {
			this.isCreatingNewEndpoint = false;
		}
	}

	async getEventDeliveriesForSidebar(eventId: string) {
		const response = await this.eventDeliveriesRequest({
			eventId,
			startDate: '',
			endDate: ''
		});
		this.sidebarEventDeliveries = response.data.content;
	}

	async toggleActiveGroup() {
		await Promise.all([this.clearEventFilters('event deliveries'), this.clearEventFilters('events')]);
		Promise.all([this.getEvents(), this.getEventDeliveries()]);
	}

	async getDelieveryAttempts(eventDeliveryId: string) {
		try {
			const deliveryAttemptsResponse = await this.convyAppService.request({
				url: this.getAPIURL(`/eventdeliveries/${eventDeliveryId}/deliveryattempts`),
				method: 'get',
				token: this.token
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
			await this.convyAppService.request({
				method: 'put',
				url: this.getAPIURL(`/eventdeliveries/${requestDetails.eventDeliveryId}/resend`),
				token: this.token
			});

			this.convyAppService.showNotification({
				message: 'Retry Request Sent'
			});
			retryButton.classList.remove(['spin', 'disabled']);
			retryButton.disabled = false;
			this.getEventDeliveries();
		} catch (error: any) {
			this.convyAppService.showNotification({
				message: error.error.message
			});
			retryButton.classList.remove(['spin', 'disabled']);
			retryButton.disabled = false;
			return error;
		}
	}

	async batchRetryEvent() {
		let eventDeliveryStatusFilterQuery = '';
		let eventDeliveriesStatusFilterActive = false;
		this.eventDeliveryFilteredByStatus.length > 0 ? (eventDeliveriesStatusFilterActive = true) : (eventDeliveriesStatusFilterActive = false);
		this.eventDeliveryFilteredByStatus.forEach((status: string) => (eventDeliveryStatusFilterQuery += `&status=${status}`));
		const { startDate, endDate } = this.setDateForFilter(this.eventDeliveriesFilterDateRange.value);
		this.isRetyring = true;
		try {
			const response = await this.convyAppService.request({
				method: 'post',
				url: this.getAPIURL(
					`/eventdeliveries/batchretry?eventId=${this.eventDeliveryFilteredByEventId || ''}&page=${this.eventDeliveriesPage || 1}&startDate=${startDate}&endDate=${endDate}${
						eventDeliveryStatusFilterQuery || ''
					}`
				),
				token: this.token,
				body: null
			});

			this.convyAppService.showNotification({ message: response.message });
			this.getEventDeliveries();
			this.showBatchRetryModal = false;
			this.isRetyring = false;
		} catch (error: any) {
			this.isRetyring = false;
			this.convyAppService.showNotification({ message: error.error.message });
			return error;
		}
	}

	async fetchRetryCount() {
		let eventDeliveryStatusFilterQuery = '';
		let eventDeliveriesStatusFilterActive = false;
		this.eventDeliveryFilteredByStatus.length > 0 ? (eventDeliveriesStatusFilterActive = true) : (eventDeliveriesStatusFilterActive = false);
		this.eventDeliveryFilteredByStatus.forEach((status: string) => (eventDeliveryStatusFilterQuery += `&status=${status}`));
		const { startDate, endDate } = this.setDateForFilter(this.eventDeliveriesFilterDateRange.value);
		this.fetchingCount = true;
		try {
			const response = await this.convyAppService.request({
				url: this.getAPIURL(
					`/eventdeliveries/countbatchretryevents?eventId=${this.eventDeliveryFilteredByEventId || ''}&page=${this.eventDeliveriesPage || 1}&startDate=${startDate}&endDate=${endDate}${
						eventDeliveryStatusFilterQuery || ''
					}`
				),
				token: this.token,
				method: 'get'
			});
			this.batchRetryCount = response.data.num;
			this.fetchingCount = false;
			this.showBatchRetryModal = true;
		} catch (error: any) {
			this.fetchingCount = false;
			this.convyAppService.showNotification({ message: error.error.message });
		}
	}

	async clearEventFilters(tableName: 'events' | 'event deliveries', filterType?: 'eventsDelDate' | 'eventsDelsStatus') {
		const activeFilters = Object.assign({}, this.route.snapshot.queryParams);
		let filterItems: string[] = [];

		switch (tableName) {
			case 'events':
				this.eventApp = '';
				filterItems = ['eventsStartDate', 'eventsEndDate'];
				this.eventsFilterDateRange.patchValue({
					startDate: '',
					endDate: ''
				});
				this.selectedEventsTimeOption = '';
				this.selectedEventsDateOption = '';
				this.getEvents();
				break;

			case 'event deliveries':
				switch (filterType) {
					case 'eventsDelDate':
						filterItems = ['eventDelsStartDate', 'eventDelsEndDate'];
						break;
					case 'eventsDelsStatus':
						filterItems = ['eventDelsStatus'];
						break;
					default:
						filterItems = ['eventDelsStartDate', 'eventDelsEndDate', 'eventDelsStatus'];
						break;
				}
				this.eventDeliveriesFilterDateRange.patchValue({ startDate: '', endDate: '' });
				this.eventDeliveryFilteredByEventId = '';
				this.eventDeliveryFilteredByStatus = [];
				this.getEventDeliveries();
				break;

			default:
				break;
		}

		filterItems.forEach(key => (activeFilters.hasOwnProperty(key) ? delete activeFilters[key] : null));
		await this.router.navigate([], {
			relativeTo: this.route,
			queryParams: activeFilters
		});
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

	

	addTag() {
		const addTagInput = document.getElementById('tagInput');
		const addTagInputValue = document.getElementById('tagInput') as HTMLInputElement;
		addTagInput?.addEventListener('keydown', e => {
			if (e.which === 188) {
				if (this.eventTags.includes(addTagInputValue?.value)) {
					addTagInputValue.value = '';
					this.eventTags = this.eventTags.filter(e => String(e).trim());
				} else {
					this.eventTags.push(addTagInputValue?.value);
					addTagInputValue.value = '';
					this.eventTags = this.eventTags.filter(e => String(e).trim());
				}
				e.preventDefault();
			}
		});
	}

	removeEventTag(tag: string) {
		this.eventTags = this.eventTags.filter(e => e !== tag);
	}

	getAPIURL(url: string) {
		url = '/portal' + url;
		return !this.apiURL || this.apiURL === '' ? location.origin + url : this.apiURL + url;
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

	formatDate(date: Date) {
		return this.datePipe.transform(date, 'dd/MM/yyyy');
	}
}
