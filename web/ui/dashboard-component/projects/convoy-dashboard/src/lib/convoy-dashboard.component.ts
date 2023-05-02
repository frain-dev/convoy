import { Component, ElementRef, Input, OnInit, ViewChild } from '@angular/core';
import Chart from 'chart.js/auto';
import { APP } from './models/app.model';
import { EVENT, EVENT_DELIVERY, EVENT_DELIVERY_ATTEMPT, EVENT_TIME } from './models/event.model';
import { ActivatedRoute, Router } from '@angular/router';
import { FormArray, FormBuilder, FormControl, FormGroup, Validators } from '@angular/forms';
import { PAGINATION } from './models/global.model';
import { HTTP_RESPONSE } from './models/global.model';
import { GROUP } from './models/source.model';
import { ConvoyDashboardService } from './convoy-dashboard.service';
import { format } from 'date-fns';
import { fromEvent, Observable } from 'rxjs';
import { debounceTime, distinctUntilChanged, map, startWith, switchMap } from 'rxjs/operators';
import { DatePipe } from '@angular/common';
import { TimeFilterComponent } from './shared-components/time-filter.component';

@Component({
	selector: 'convoy-dashboard',
	templateUrl: './convoy-dashboard.component.html',
	styleUrls: ['./convoy-dashboard.component.scss']
})
export class ConvoyDashboardComponent implements OnInit {
	eventsTableHead: string[] = ['Event Type', 'App Name', 'Time Created', ''];
	eventDelTableHead: string[] = ['Status', 'Event Type', 'Attempts', 'Time Created', '', ''];
	appsTableHead: string[] = ['Status', 'Name', 'Time Created', 'Updated', 'Events', 'Endpoints'];
	showFilterCalendar = false;
	tabs: ['events', 'event deliveries', 'apps'] = ['events', 'event deliveries', 'apps'];
	activeTab: 'events' | 'apps' | 'event deliveries' = 'events';
	filterOptions: ['daily', 'weekly', 'monthly', 'yearly'] = ['daily', 'weekly', 'monthly', 'yearly'];
	appsDetailsItem?: any;
	eventsDetailsItem!: any;
	eventDelsDetailsItem?: any;
	eventDeliveryAtempt!: EVENT_DELIVERY_ATTEMPT;
	showEventFilterCalendar = false;
	showEventDelFilterCalendar = false;
	eventDateFilterActive = false;
	showFilterDropdown = false;
	displayedEvents!: {
		date: string;
		content: EVENT[];
	}[];
	events!: { pagination: PAGINATION; content: EVENT[] };
	apps!: { pagination: PAGINATION; content: APP[] };
	displayedApps: { date: string; content: APP[] }[] = [];
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
	addNewAppForm: FormGroup = this.formBuilder.group({
		name: ['', Validators.required],
		support_email: [''],
		is_disabled: [false],
		endpoints: this.formBuilder.array([])
	});
	addNewEndpointForm: FormGroup = this.formBuilder.group({
		url: ['', Validators.required],
		events: [''],
		description: ['', Validators.required]
	});
	sendEventForm: FormGroup = this.formBuilder.group({
		app_id: ['', Validators.required],
		data: ['', Validators.required],
		event_type: ['', Validators.required]
	});
	selectedEventsFromEventDeliveriesTable: string[] = [];
	displayedEventDeliveries: { date: string; content: EVENT_DELIVERY[] }[] = [];
	eventDeliveries!: { pagination: PAGINATION; content: EVENT_DELIVERY[] };
	sidebarEventDeliveries!: EVENT_DELIVERY[];
	eventDeliveryFilteredByEventId = '';
	groups: GROUP[] = [];
	allEventdeliveriesChecked = false;
	eventDeliveryStatuses = ['Success', 'Failure', 'Retry', 'Scheduled', 'Processing', 'Discarded'];
	dateOptions = ['Last Year', 'Last Month', 'Last Week', 'Yesterday'];
	appStatuses = ['All', 'Enabled', 'Disabled'];
	eventDeliveryFilteredByStatus: string[] = [];
	eventTags: string[] = [];
	showOverlay = false;
	showEventDeliveriesStatusDropdown = false;
	showEventDeliveriesAppsDropdown = false;
	showEventsAppsDropdown = false;
	showCreateAppModal = false;
	showAddEndpointModal = false;
	showAddEventModal = false;
	showDeleteAppModal = false;
	showBatchRetryModal = false;
	showDateFilterDropdown = false;
	editAppMode = false;
	loadingAppPotalToken = false;
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
	isloadingDeliveryAttempts = true;
	isCreatingNewApp = false;
	isCreatingNewEndpoint = false;
	isSendingNewEvent = false;
	isDeletingApp = false;
	isRetyring = false;
	fetchingCount = false;
	updateAppDetail = false;
	showPublicCopyText = false;
	showSecretCopyText = false;
	showEndpointSecret = false;
	renderDashboard = true;
	appsSearchString = '';
	eventsSearchString = '';
	selectedEventsDateOption = '';
	selectedEventsDelDateOption = '';
	selectedDateOption = '';
	currentAppId = '';
	tag = '';
	eventDeliveryIndex!: number;
	appPortalLink!: string;
	endpointSecretKey = '';
	selectedAppStatus = 'All';
	batchRetryCount!: any;
	eventsAppsFilter$!: Observable<APP[]>;
	eventsDelAppsFilter$!: Observable<APP[]>;
	@ViewChild('eventsAppsFilter', { static: true }) eventsAppsFilter!: ElementRef;
	@ViewChild('eventDelsAppsFilter', { static: true }) eventDelsAppsFilter!: ElementRef;
	eventDeliveriesStatusFilterActive = false;
	eventsTimeFilterData: { startTime: string; endTime: string } = { startTime: 'T00:00:00', endTime: 'T23:59:59' };
	eventDelsTimeFilterData: { startTime: string; endTime: string } = { startTime: 'T00:00:00', endTime: 'T23:59:59' };
	@ViewChild('eventsTimeFilter', { static: true }) eventsTimerFilter!: TimeFilterComponent;
	@ViewChild('eventDeliveryTimerFilter', { static: true }) eventDeliveryTimerFilter!: TimeFilterComponent;
	showProjectsModal = !!this.convyDashboardService.activeGroupId;
	isLoadingProjects = false;
	groupsLoaderIndex: number[] = [0, 1, 2, 3];

	constructor(public convyDashboardService: ConvoyDashboardService, private router: Router, private formBuilder: FormBuilder, private route: ActivatedRoute, private datePipe: DatePipe) {}

	async ngOnInit() {
		this.convyDashboardService.authType = this.apiAuthType;
		this.convyDashboardService.url = this.apiURL;
		this.convyDashboardService.token = this.requestToken;

		if (!this.requestToken || this.requestToken == '') {
			this.convyDashboardService.showNotification({ message: 'You are not logged in', style: 'warning' });
			return this.router.navigate(['/login']);
		}

		if (!this.apiURL) return this.convyDashboardService.showNotification({ message: 'Please provide API URL for Convoy dashboard component.', style: 'warning' });
		if (this.isCloud && !this.groupId) return this.convyDashboardService.showNotification({ message: 'Please provide group ID for Convoy dashboard component.', style: 'warning' });
		if (this.isCloud) {
			this.convyDashboardService.activeGroupId = this.groupId;
			this.apiAuthType = 'Bearer';
			this.convyDashboardService.authType = this.apiAuthType;
		}

		return await this.initDashboard();
	}

	// seach filters for apps on events and events delivery tab
	ngAfterViewInit() {
		this.eventsAppsFilter$ = fromEvent<any>(this.eventsAppsFilter?.nativeElement, 'keyup').pipe(
			map(event => event.target.value),
			startWith(''),
			debounceTime(500),
			distinctUntilChanged(),
			switchMap(search => this.getAppsForFilter(search))
		);
		this.eventsDelAppsFilter$ = fromEvent<any>(this.eventDelsAppsFilter?.nativeElement, 'keyup').pipe(
			map(event => event.target.value),
			startWith(''),
			debounceTime(500),
			distinctUntilChanged(),
			switchMap(search => this.getAppsForFilter(search))
		);
	}

	get endpoints(): FormArray {
		return this.addNewAppForm.get('endpoints') as FormArray;
	}

	getSingleEndpoint(index: any) {
		return ((this.addNewAppForm.get('endpoints') as FormArray)?.controls[index] as FormGroup)?.controls;
	}

	newEndpoint(): FormGroup {
		return this.formBuilder.group({
			url: ['', Validators.required],
			events: [''],
			tag: ['', Validators.required],
			description: ['', Validators.required]
		});
	}

	addEndpoint() {
		this.endpoints.push(this.newEndpoint());
	}

	removeEndpoint(i: number) {
		this.endpoints.removeAt(i);
	}

	// add tag function for adding multiple events to input form, <to be reviewed>
	addTag(event?: any, i?: any) {
		// to be reviewed
		// let eventTagControlNames = [];
		// const tagControlName = event.target.getAttribute('formcontrolname');
		// const eventTagControlName = `${tagControlName} ${i}`;
		// eventTagControlNames.push(eventTagControlName);
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

	// create/edit new application function
	async createNewApp() {
		if (this.addNewAppForm.invalid) {
			(<any>Object).values(this.addNewAppForm.controls).forEach((control: FormControl) => {
				control?.markAsTouched();
			});
			return;
		}

		this.isCreatingNewApp = true;
		// to be reviewed
		delete this.addNewAppForm.value.endpoints;

		try {
			const response = this.editAppMode
				? await this.convyDashboardService.updateApp({ appId: this.appsDetailsItem?.uid, body: this.addNewAppForm.value })
				: await this.convyDashboardService.createApp({ body: this.addNewAppForm.value });

			if (this.editAppMode) this.updateAppDetail = true;
			this.convyDashboardService.showNotification({ message: response.message, style: 'success' });
			this.addNewAppForm.reset();
			this.getApps({ type: 'apps' });
			this.showCreateAppModal = false;
			this.isCreatingNewApp = false;
			this.editAppMode = false;
			return;
		} catch (error) {
			this.isCreatingNewApp = false;
			return;
		}
	}

	async deleteApp() {
		this.isDeletingApp = true;
		try {
			const response = await this.convyDashboardService.deleteApp({ appId: this.appsDetailsItem?.uid });
			// this.appsDetailsItem = {};
			this.convyDashboardService.showNotification({ message: response.message, style: 'success' });
			this.getApps({ type: 'apps' });
			this.toggleActiveTab('apps');
			this.showDeleteAppModal = false;
			this.isDeletingApp = false;
		} catch {
			this.isDeletingApp = false;
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
			const response = await this.convyDashboardService.addNewEndpoint({ appId: appUid ? appUid : this.appsDetailsItem?.uid, body: this.addNewEndpointForm.value });
			this.convyDashboardService.showNotification({ message: response.message, style: 'success' });
			this.getApps({ type: 'apps' });
			this.updateAppDetail = true;
			this.addNewEndpointForm.reset();
			this.eventTags = [];
			this.showAddEndpointModal = false;
			this.isCreatingNewEndpoint = false;
			return;
		} catch {
			this.isCreatingNewEndpoint = false;
			return;
		}
	}

	async sendNewEvent() {
		if (this.sendEventForm.invalid) {
			(<any>Object).values(this.sendEventForm.controls).forEach((control: FormControl) => {
				control?.markAsTouched();
			});
			return;
		}
		this.isSendingNewEvent = true;
		try {
			const response = await this.convyDashboardService.sendEvent({ body: this.sendEventForm.value });

			this.convyDashboardService.showNotification({ message: response.message, style: 'success' });
			this.getEventDeliveries();
			this.getEvents();
			this.toggleActiveTab('event deliveries');
			this.sendEventForm.reset();
			this.toggleDashboard();
			this.showAddEventModal = false;
			this.isSendingNewEvent = false;
		} catch {
			this.isSendingNewEvent = false;
		}
	}

	async fetchRetryCount() {
		let eventDeliveryStatusFilterQuery = '';
		this.eventDeliveryFilteredByStatus.length > 0 ? (this.eventDeliveriesStatusFilterActive = true) : (this.eventDeliveriesStatusFilterActive = false);
		this.eventDeliveryFilteredByStatus.forEach((status: string) => (eventDeliveryStatusFilterQuery += `&status=${status}`));
		const { startDate, endDate } = this.setDateForFilter(this.eventDeliveriesFilterDateRange.value);
		this.fetchingCount = true;
		try {
			const response = await this.convyDashboardService.getRetryCount({
				eventId: this.eventDeliveryFilteredByEventId || '',
				pageNo: this.eventDeliveriesPage || 1,
				startDate: startDate,
				endDate: endDate,
				appId: this.eventDeliveriesApp,
				statusQuery: eventDeliveryStatusFilterQuery || ''
			});

			this.batchRetryCount = response.data.num;
			this.fetchingCount = false;
			this.showBatchRetryModal = true;
		} catch (error: any) {
			this.fetchingCount = false;
			this.convyDashboardService.showNotification({ message: error.error.message, style: 'error' });
		}
	}

	removeEventTag(tag: string) {
		this.eventTags = this.eventTags.filter(e => e !== tag);
	}

	openUpdateAppModal(app: APP) {
		this.showCreateAppModal = true;
		this.editAppMode = true;
		this.currentAppId = '';
		this.addNewAppForm.patchValue({
			name: app.name,
			support_email: app.support_email,
			is_disabled: app.is_disabled
		});
	}

	editAppStatus(app: APP) {
		let isDisabled;
		if (app.is_disabled) {
			isDisabled = false;
		} else {
			isDisabled = true;
		}
		this.addNewAppForm.patchValue({
			name: app.name,
			support_email: app.support_email,
			is_disabled: isDisabled
		});
		this.editAppMode = true;
		this.createNewApp();
	}

	setEventAppId() {
		this.showAddEventModal = !this.showAddEventModal;
		this.sendEventForm.patchValue({
			app_id: this.appsDetailsItem?.uid
		});
	}

	// copy code snippet
	copyKey(key: string, type: 'public' | 'secret') {
		const text = key;
		const el = document.createElement('textarea');
		el.value = text;
		document.body.appendChild(el);
		el.select();
		document.execCommand('copy');
		type === 'public' ? (this.showPublicCopyText = true) : (this.showSecretCopyText = true);
		setTimeout(() => {
			type === 'public' ? (this.showPublicCopyText = false) : (this.showSecretCopyText = false);
		}, 3000);
		document.body.removeChild(el);
	}

	getSelectedDate(dateOption: string, activeTab?: string) {
		if (activeTab) {
			activeTab == 'events' ? (this.selectedEventsDateOption = dateOption) : (this.selectedEventsDelDateOption = dateOption);
		} else {
			this.selectedDateOption = dateOption;
		}
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
			this.getEvents({ addToURL: true, fromFilter: true });
		} else if (activeTab == 'event deliveries') {
			this.eventDeliveriesFilterDateRange.patchValue({
				startDate: startDate,
				endDate: endDate
			});
			this.getEventDeliveries({ addToURL: true, fromFilter: true });
		} else {
			this.statsDateRange.patchValue({
				startDate: startDate,
				endDate: endDate
			});
			this.fetchDashboardData();
		}
	}

	// initiate dashboard
	async initDashboard() {
		await this.getGroups();
		this.getFiltersFromURL();
		this.activeTab = this.route.snapshot.queryParams?.activeTab ?? 'events';
		await Promise.all([this.getConfigDetails(), this.fetchDashboardData(), this.getEvents({ fromFilter: true }), this.getApps({ type: 'apps' }), this.getEventDeliveries()]);

		// get active tab from url and apply, after getting the details from above requests so that the data is available ahead
		this.toggleActiveTab(this.route.snapshot.queryParams?.activeTab ?? 'events');
		return;
	}

	async toggleActiveTab(tab: 'events' | 'apps' | 'event deliveries') {
		this.activeTab = tab;
		this.addFilterToURL({ section: 'logTab' });

		if (tab === 'apps' && this.apps?.content.length > 0) {
			if (!this.appsDetailsItem) {
				this.appsDetailsItem = this.apps?.content[0];
				await this.getAppPortalToken({ redirect: false });
			}
			return;
		} else if (tab === 'events' && this.events?.content.length > 0) {
			if (!this.eventsDetailsItem) this.eventsDetailsItem = this.events?.content[0];
			if (this.eventsDetailsItem?.uid) await this.getEventDeliveriesForSidebar(this.eventsDetailsItem.uid);
			return;
		} else if (tab === 'event deliveries' && this.eventDeliveries?.content.length > 0) {
			if (!this.eventDelsDetailsItem) this.eventDelsDetailsItem = this.eventDeliveries?.content[0];
			if (this.eventDelsDetailsItem?.uid) await this.getDelieveryAttempts(this.eventDelsDetailsItem.uid);
			return;
		}
	}

	async getConfigDetails(): Promise<HTTP_RESPONSE> {
		this.isloadingConfig = true;

		try {
			const organisationDetailsResponse = await this.convyDashboardService.getConfigDetails();
			this.organisationDetails = organisationDetailsResponse.data;
			this.isloadingConfig = false;

			return organisationDetailsResponse;
		} catch (error: any) {
			this.isloadingConfig = false;
			return error;
		}
	}

	setTimeFilterData(dates: EVENT_TIME): { startTime: string; endTime: string } {
		const response = { startTime: '', endTime: '' };
		if (dates.startDate) {
			const hour = new Date(dates.startDate).getHours();
			const minute = new Date(dates.startDate).getMinutes();

			dates.type === 'events' ? (this.eventsTimerFilter.filterStartHour = hour) : (this.eventDeliveryTimerFilter.filterStartHour = hour);
			dates.type === 'events' ? (this.eventsTimerFilter.filterStartMinute = minute) : (this.eventDeliveryTimerFilter.filterStartMinute = minute);

			response.startTime = `T${hour}:${minute}:00`;
		} else {
			response.startTime = 'T00:00:00';
		}

		if (dates.endDate) {
			const hour = new Date(dates.endDate).getHours();
			const minute = new Date(dates.endDate).getMinutes();

			dates.type === 'events' ? (this.eventsTimerFilter.filterEndHour = hour) : (this.eventDeliveryTimerFilter.filterEndHour = hour);
			dates.type === 'events' ? (this.eventsTimerFilter.filterEndMinute = minute) : (this.eventDeliveryTimerFilter.filterEndMinute = minute);

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

		// for events filters
		this.eventsFilterDateRange.patchValue({ startDate: filters.eventsStartDate ? new Date(filters.eventsStartDate) : '', endDate: filters.eventsEndDate ? new Date(filters.eventsEndDate) : '' });
		this.eventApp = filters.eventsApp ?? '';
		this.eventsSearchString = filters.eventsSearch ?? '';
		const eventsTimeFilter = this.setTimeFilterData({ startDate: filters?.eventsStartDate, endDate: filters?.eventsEndDate, type: 'events' });
		this.eventsTimeFilterData = { ...eventsTimeFilter };

		// for event deliveries filters
		this.eventDeliveriesFilterDateRange.patchValue({
			startDate: filters.eventDelsStartDate ? new Date(filters.eventDelsStartDate) : '',
			endDate: filters.eventDelsEndDate ? new Date(filters.eventDelsEndDate) : ''
		});

		// for viewing app/event delivery details
		filters.renderDashboard === 'true' ? (this.renderDashboard = true) : (this.renderDashboard = false);

		this.eventDeliveriesApp = filters.eventDelsApp ?? '';
		this.eventDeliveryFilteredByStatus = filters.eventDelsStatus ? JSON.parse(filters.eventDelsStatus) : [];
		const eventDeliveriesTimeFilter = this.setTimeFilterData({ startDate: filters?.eventDelsStartDate, endDate: filters?.eventDelsEndDate, type: 'eventDeliveries' });
		this.eventsTimeFilterData = { ...eventDeliveriesTimeFilter };
	}

	async fetchDashboardData(): Promise<HTTP_RESPONSE> {
		try {
			this.isloadingDashboardData = true;
			const { startDate, endDate } = this.setDateForFilter(this.statsDateRange.value);

			const dashboardResponse = await this.convyDashboardService.dashboardSummary({ startDate: startDate || '', endDate: endDate || '', frequency: this.dashboardFrequency });
			this.dashboardData = dashboardResponse.data;
			this.isloadingDashboardData = false;
			this.initChart(dashboardResponse);

			return dashboardResponse;
		} catch (error: any) {
			this.isloadingDashboardData = false;
			return error;
		}
	}

	initChart(dashboardResponse: HTTP_RESPONSE) {
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
	}

	setDateForFilter(requestDetails: { startDate: Date; endDate: Date; startTime?: string; endTime?: string }) {
		if (!requestDetails.endDate && !requestDetails.startDate) return { startDate: '', endDate: '' };
		const startDate = requestDetails.startDate ? `${format(requestDetails.startDate, 'yyyy-MM-dd')}${requestDetails?.startTime || 'T00:00:00'}` : '';
		const endDate = requestDetails.endDate ? `${format(requestDetails.endDate, 'yyyy-MM-dd')}${requestDetails?.endTime || 'T23:59:59'}` : '';
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

	setContentDisplayed(content: { created_at: Date }[]) {
		const dateCreateds = content.map((item: { created_at: Date }) => this.getDate(item.created_at));
		const uniqueDateCreateds = [...new Set(dateCreateds)];
		const displayedItems: any = [];
		uniqueDateCreateds.forEach(itemDate => {
			const filteredItemDate = content.filter((item: { created_at: Date }) => this.getDate(item.created_at) === itemDate);
			const contents = { date: itemDate, content: filteredItemDate };
			displayedItems.push(contents);
		});
		return displayedItems;
	}

	async getEvents(requestDetails?: { appId?: string; addToURL?: boolean; fromFilter?: boolean }): Promise<HTTP_RESPONSE> {
		this.events && this.events?.pagination?.next === this.eventsPage ? (this.isloadingMoreEvents = true) : (this.isloadingEvents = true);
		if (requestDetails?.appId) this.eventApp = requestDetails.appId;
		if (requestDetails?.addToURL) this.addFilterToURL({ section: 'events' });

		if (this.eventsSearchString && this.eventsPage === 1) this.displayedEvents = [];

		const { startDate, endDate } = this.setDateForFilter({ ...this.eventsFilterDateRange.value, ...this.eventsTimeFilterData });

		try {
			const eventsResponse = await this.convyDashboardService.getEvents({
				pageNo: this.eventsPage || 1,
				startDate,
				endDate,
				appId: requestDetails?.appId ?? this.eventApp,
				query: this.eventsSearchString || ''
			});

			if (this.events && this.events?.pagination?.next === this.eventsPage) {
				const content = [...this.events.content, ...eventsResponse.data.content];
				const pagination = eventsResponse.data.pagination;
				this.events = { content, pagination };
				this.displayedEvents = this.setContentDisplayed(content);
				this.isloadingMoreEvents = false;
				return eventsResponse;
			}

			this.events = eventsResponse.data;
			this.displayedEvents = await this.setContentDisplayed(eventsResponse.data.content);

			// if this is a filter request, set the eventsDetailsItem to the first item in the list
			if (requestDetails?.fromFilter) {
				this.eventsDetailsItem = this.events?.content[0];
				this.getEventDeliveriesForSidebar(this.eventsDetailsItem.uid);
			}

			this.isloadingEvents = false;
			return eventsResponse;
		} catch (error: any) {
			this.isloadingEvents = false;
			this.isloadingMoreEvents = false;
			return error;
		}
	}

	async getAppPortalToken(requestDetail: { redirect: boolean }) {
		this.loadingAppPotalToken = true;

		try {
			const appTokenResponse = await this.convyDashboardService.getAppPortalToken({ appId: this.appsDetailsItem.uid });
			this.appPortalLink = `<iframe style="width: 100%; height: 100vh; border: none;" src="${appTokenResponse.data.url}"></iframe>`;
			if (requestDetail.redirect) window.open(`${appTokenResponse.data.url}`, '_blank');
			this.loadingAppPotalToken = false;
		} catch (error) {
			this.loadingAppPotalToken = false;
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
			const { startDate, endDate } = this.setDateForFilter({ ...this.eventsFilterDateRange.value, ...this.eventsTimeFilterData });
			if (startDate) queryParams.eventsStartDate = startDate;
			if (endDate) queryParams.eventsEndDate = endDate;
			if (this.eventApp) queryParams.eventsApp = this.eventApp;
			if (this.eventsSearchString) queryParams.eventsSearch = this.eventsSearchString;
		}

		if (requestDetails.section === 'eventDels') {
			const { startDate, endDate } = this.setDateForFilter({ ...this.eventDeliveriesFilterDateRange.value, ...this.eventDelsTimeFilterData });
			if (startDate) queryParams.eventDelsStartDate = startDate;
			if (endDate) queryParams.eventDelsEndDate = endDate;
			if (this.eventDeliveriesApp) queryParams.eventDelsApp = this.eventDeliveriesApp;
			queryParams.eventDelsStatus = this.eventDeliveryFilteredByStatus.length > 0 ? JSON.stringify(this.eventDeliveryFilteredByStatus) : '';
		}

		if (requestDetails.section === 'group') queryParams.group = this.convyDashboardService.activeGroupId;

		if (requestDetails.section === 'logTab') queryParams.activeTab = this.activeTab;

		this.router.navigate([], { queryParams: Object.assign({}, currentURLfilters, queryParams) });
	}

	async eventDeliveriesRequest(requestDetails: { eventId?: string; startDate?: string; endDate?: string }): Promise<HTTP_RESPONSE> {
		let eventDeliveryStatusFilterQuery = '';
		this.eventDeliveryFilteredByStatus.length > 0 ? (this.eventDeliveriesStatusFilterActive = true) : (this.eventDeliveriesStatusFilterActive = false);
		this.eventDeliveryFilteredByStatus.forEach((status: string) => (eventDeliveryStatusFilterQuery += `&status=${status}`));

		try {
			const eventDeliveriesResponse = await this.convyDashboardService.getEventDeliveries({
				eventId: requestDetails.eventId || '',
				pageNo: this.eventDeliveriesPage || 1,
				startDate: requestDetails.startDate,
				endDate: requestDetails.endDate,
				appId: this.eventDeliveriesApp,
				statusQuery: eventDeliveryStatusFilterQuery || ''
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

	async getEventDeliveries(requestDetails?: { addToURL?: boolean; fromFilter?: boolean }): Promise<HTTP_RESPONSE> {
		this.eventDeliveries && this.eventDeliveries?.pagination?.next === this.eventDeliveriesPage ? (this.isloadingMoreEventDeliveries = true) : (this.isloadingEventDeliveries = true);

		if (requestDetails?.addToURL) this.addFilterToURL({ section: 'eventDels' });
		const { startDate, endDate } = this.setDateForFilter({ ...this.eventDeliveriesFilterDateRange.value, ...this.eventDelsTimeFilterData });

		try {
			const eventDeliveriesResponse = await this.eventDeliveriesRequest({ eventId: this.eventDeliveryFilteredByEventId, startDate, endDate });

			if (this.eventDeliveries && this.eventDeliveries?.pagination?.next === this.eventDeliveriesPage) {
				const content = [...this.eventDeliveries.content, ...eventDeliveriesResponse.data.content];
				const pagination = eventDeliveriesResponse.data.pagination;
				this.eventDeliveries = { content, pagination };
				this.displayedEventDeliveries = this.setContentDisplayed(content);
				this.isloadingMoreEventDeliveries = false;
				return eventDeliveriesResponse;
			}

			this.eventDeliveries = eventDeliveriesResponse.data;
			this.displayedEventDeliveries = this.setContentDisplayed(eventDeliveriesResponse.data.content);

			// if this is a filter request, set the eventDelsDetailsItem to the first item in the list
			if (requestDetails?.fromFilter) {
				this.eventDelsDetailsItem = this.eventDeliveries?.content[0];
				this.getDelieveryAttempts(this.eventDelsDetailsItem.uid);
			}

			this.isloadingEventDeliveries = false;
			return eventDeliveriesResponse;
		} catch (error: any) {
			this.isloadingEventDeliveries = false;
			this.isloadingMoreEventDeliveries = false;
			return error;
		}
	}

	async getEventDeliveriesForSidebar(eventId: string) {
		const response = await this.convyDashboardService.getEventDeliveries({
			eventId,
			startDate: '',
			endDate: '',
			pageNo: 1,
			appId: '',
			statusQuery: ''
		});
		this.sidebarEventDeliveries = response.data.content;
	}

	async toggleActiveGroup() {
		await Promise.all([this.clearEventFilters('event deliveries'), this.clearEventFilters('events')]);
		this.addFilterToURL({ section: 'group' });
		this.eventsDetailsItem = null;
		this.eventDelsDetailsItem = null;
		this.appsDetailsItem = null;
		await Promise.all([this.getConfigDetails(), this.fetchDashboardData(), this.getEvents(), this.getApps({ type: 'apps' }), this.getEventDeliveries()]);
	}

	async getGroups(requestDetails?: { addToURL?: boolean }): Promise<HTTP_RESPONSE> {
		this.isLoadingProjects = true;
		if (requestDetails?.addToURL) this.addFilterToURL({ section: 'group' });

		try {
			const groupsResponse = await this.convyDashboardService.getGroups();
			this.groups = groupsResponse.data;

			// check group existing filter in url and set active group
			if (!this.isCloud) this.convyDashboardService.activeGroupId = this.route.snapshot.queryParams?.group ?? this.groups[0]?.uid;
			this.isLoadingProjects = false;
			return groupsResponse;
		} catch (error: any) {
			this.isLoadingProjects = false;
			return error;
		}
	}

	async getAppsForFilter(search: string): Promise<APP[]> {
		return await (
			await this.convyDashboardService.getApps({ pageNo: this.appsPage || 1, searchString: search })
		).data.content;
	}

	filterAppByStatus(status: string) {
		this.selectedAppStatus = status;
	}

	async getApps(requestDetails?: { search?: string; type: 'filter' | 'apps' }): Promise<HTTP_RESPONSE> {
		if (this.apps?.pagination?.next === this.appsPage) this.isloadingMoreApps = true;
		if (requestDetails?.type === 'apps') this.isloadingApps = true;

		try {
			const appsResponse = await this.convyDashboardService.getApps({ pageNo: this.appsPage || 1, searchString: requestDetails?.search });

			if (!requestDetails?.search && this.apps?.pagination?.next === this.appsPage) {
				const content = [...this.apps.content, ...appsResponse.data.content];
				const pagination = appsResponse.data.pagination;
				this.apps = { content, pagination };
				this.displayedApps = this.setContentDisplayed(this.apps.content);
				this.isloadingMoreApps = false;
				return appsResponse;
			}

			if (requestDetails?.type === 'apps') {
				this.apps = appsResponse.data;
				this.displayedApps = this.setContentDisplayed(this.apps.content);
				this.appsDetailsItem = this.apps?.content[0];
				this.getAppPortalToken({ redirect: false });
			}

			if (!this.filteredApps) this.filteredApps = appsResponse.data.content;

			if (this.updateAppDetail) this.appsDetailsItem = this.apps.content.find(item => this.appsDetailsItem?.uid == item.uid);

			this.isloadingApps = false;
			return appsResponse;
		} catch (error: any) {
			this.isloadingApps = false;
			this.isloadingMoreApps = false;
			return error;
		}
	}

	async getDelieveryAttempts(eventDeliveryId: string) {
		this.isloadingDeliveryAttempts = true;
		try {
			const deliveryAttemptsResponse = await this.convyDashboardService.getEventDeliveryAttempts({ eventDeliveryId });
			this.eventDeliveryAtempt = deliveryAttemptsResponse.data[deliveryAttemptsResponse.data.length - 1];
			this.isloadingDeliveryAttempts = false;

			return;
		} catch (error) {
			this.isloadingDeliveryAttempts = false;
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
			await this.convyDashboardService.retryEvent({ eventId: requestDetails.eventDeliveryId });

			this.convyDashboardService.showNotification({ message: 'Retry Request Sent', style: 'success' });
			retryButton.classList.remove(['spin', 'disabled']);
			retryButton.disabled = false;
			this.getEventDeliveries();
		} catch (error: any) {
			this.convyDashboardService.showNotification({ message: `${error?.error?.message ? error?.error?.message : 'An error occured'}`, style: 'error' });
			if (retryButton) {
				retryButton.classList.remove(['spin', 'disabled']);
				retryButton.disabled = false;
			}
			return error;
		}
	}

	// force retry successful events
	async forceRetryEvent(requestDetails: { e: any; index: number; eventDeliveryId: string }) {
		requestDetails.e.stopPropagation();
		const retryButton: any = document.querySelector(`#event${requestDetails.index} button`);
		if (retryButton) {
			retryButton.classList.add(['spin', 'disabled']);
			retryButton.disabled = true;
		}
		const payload = {
			ids: [requestDetails.eventDeliveryId]
		};
		try {
			await this.convyDashboardService.forceRetryEvent({ body: payload });

			this.convyDashboardService.showNotification({ message: 'Force Retry Request Sent', style: 'success' });
			retryButton.classList.remove(['spin', 'disabled']);
			retryButton.disabled = false;
			this.getEventDeliveries();
		} catch (error: any) {
			this.convyDashboardService.showNotification({ message: `${error?.error?.message ? error?.error?.message : 'An error occured'}`, style: 'error' });
			if (retryButton) {
				retryButton.classList.remove(['spin', 'disabled']);
				retryButton.disabled = false;
			}
			return error;
		}
	}

	async batchRetryEvent() {
		let eventDeliveryStatusFilterQuery = '';
		this.eventDeliveryFilteredByStatus.length > 0 ? (this.eventDeliveriesStatusFilterActive = true) : (this.eventDeliveriesStatusFilterActive = false);
		this.eventDeliveryFilteredByStatus.forEach((status: string) => (eventDeliveryStatusFilterQuery += `&status=${status}`));
		const { startDate, endDate } = this.setDateForFilter(this.eventDeliveriesFilterDateRange.value);
		this.isRetyring = true;
		try {
			const response = await this.convyDashboardService.batchRetryEvent({
				eventId: this.eventDeliveryFilteredByEventId || '',
				pageNo: this.eventDeliveriesPage || 1,
				startDate: startDate,
				endDate: endDate,
				appId: this.eventDeliveriesApp,
				statusQuery: eventDeliveryStatusFilterQuery || ''
			});

			this.convyDashboardService.showNotification({ message: response.message, style: 'success' });
			this.getEventDeliveries();
			this.showBatchRetryModal = false;
			this.isRetyring = false;
		} catch (error: any) {
			this.isRetyring = false;
			this.convyDashboardService.showNotification({ message: error?.error?.message, style: 'error' });
			return error;
		}
	}

	async clearEventFilters(tableName: 'events' | 'event deliveries' | 'apps', filterType?: 'eventsDate' | 'eventsDelDate' | 'eventsApp' | 'eventsDelApp' | 'eventsDelsStatus' | 'eventsSearch') {
		const activeFilters = Object.assign({}, this.route.snapshot.queryParams);
		let filterItems: string[] = [];

		switch (tableName) {
			case 'events':
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
				this.getEvents({ fromFilter: true });
				break;

			case 'event deliveries':
				this.eventDeliveriesApp = '';
				switch (filterType) {
					case 'eventsDelApp':
						filterItems = ['eventDelsApp'];
						break;
					case 'eventsDelDate':
						filterItems = ['eventDelsStartDate', 'eventDelsEndDate'];
						break;
					case 'eventsDelsStatus':
						filterItems = ['eventDelsStatus'];
						break;
					default:
						filterItems = ['eventDelsStartDate', 'eventDelsEndDate', 'eventDelsApp', 'eventDelsStatus'];
						break;
				}
				filterItems = ['eventDelsStartDate', 'eventDelsEndDate', 'eventDelsApp', 'eventDelsStatus'];
				this.eventDeliveriesFilterDateRange.patchValue({ startDate: '', endDate: '' });
				this.eventDeliveryFilteredByEventId = '';
				this.eventDeliveryFilteredByStatus = [];
				this.eventDelsTimeFilterData = { startTime: 'T00:00:00', endTime: 'T23:59:59' };
				this.eventsTimerFilter.clearFilter();
				this.getEventDeliveries({ fromFilter: true });
				break;
			case 'apps':
				this.selectedAppStatus = 'All';
				this.getApps({ type: 'apps' });
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

	async openAppsTabAndDetails(appName: string) {
		this.appsSearchString = appName;
		this.toggleActiveTab('apps');
		this.searchApps({ type: 'apps' });
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

	formatDate(date: Date) {
		return this.datePipe.transform(date, 'dd/MM/yyyy');
	}

	// check if string contains special character
	containsSpecialCharacters(str: string) {
		const specialChars = /[`!@#$%^&*()_+\-=\[\]{};':"\\|,.<>\/?~]/;
		return specialChars.test(str);
	}

	selectedGroupName() {
		return this.groups.find(item => item.uid === this.convyDashboardService.activeGroupId)?.name;
	}

	viewEndpointSecretKey(secretKey: string) {
		this.showEndpointSecret = !this.showEndpointSecret;
		this.endpointSecretKey = secretKey;
	}

	toggleDashboard() {
		const currentURLfilters = this.route.snapshot.queryParams;
		let queryParams: any = {};
		if (this.renderDashboard) {
			this.renderDashboard = false;
			queryParams = { renderDashboard: false };
		} else {
			this.renderDashboard = true;
			queryParams = { renderDashboard: true };
		}
		this.router.navigate([], { queryParams: Object.assign({}, currentURLfilters, queryParams) });
	}
}
