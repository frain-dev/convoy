import { DatePipe } from '@angular/common';
import { Component, ElementRef, Input, OnInit, ViewChild } from '@angular/core';
import { FormBuilder, FormGroup } from '@angular/forms';
import { ActivatedRoute, Router } from '@angular/router';
import { format } from 'date-fns';
import { fromEvent, Observable } from 'rxjs';
import { debounceTime, distinctUntilChanged, map, startWith, switchMap } from 'rxjs/operators';
import { APP } from 'src/app/models/app.model';
import { EVENT_DELIVERY, EVENT_DELIVERY_ATTEMPT } from 'src/app/models/event.model';
import { PAGINATION } from 'src/app/models/global.model';
import { HTTP_RESPONSE } from 'src/app/models/http.model';
import { TimeFilterComponent } from 'src/app/private/components/time-filter/time-filter.component';
import { GeneralService } from 'src/app/services/general/general.service';
import { EventsService } from '../events.service';

@Component({
	selector: 'app-event-deliveries',
	templateUrl: './event-deliveries.component.html',
	styleUrls: ['../events.component.scss']
})
export class EventDeliveriesComponent implements OnInit {
	@Input() eventDeliveryFilteredByEventId!: string;
	dateOptions = ['Last Year', 'Last Month', 'Last Week', 'Yesterday'];
	eventDeliveryStatuses = ['Success', 'Failure', 'Retry', 'Scheduled', 'Processing', 'Discarded'];
	eventDelTableHead: string[] = ['Status', 'Event Type', 'Attempts', 'Time Created', '', ''];
	showEventDelFilterCalendar = false;
	showEventDeliveriesStatusDropdown = false;
	eventDeliveriesStatusFilterActive = false;
	showEventDeliveriesAppsDropdown = false;
	showOverlay = false;
	fetchingCount = false;
	showBatchRetryModal = false;
	isloadingEventDeliveries = false;
	isloadingDeliveryAttempts = false;
	isRetrying = false;
	eventDeliveriesFilterDateRange: FormGroup = this.formBuilder.group({
		startDate: [{ value: '', disabled: true }],
		endDate: [{ value: '', disabled: true }]
	});
	batchRetryCount!: number;
	eventDeliveriesApp!: string;
	eventDelsDetailsItem?: any;
	eventDeliveryIndex!: number;
	eventDeliveriesPage: number = 1;
	selectedEventsFromEventDeliveriesTable: string[] = [];
	displayedEventDeliveries: { date: string; content: EVENT_DELIVERY[] }[] = [
		{
			date: '28 Mar, 2022',
			content: [
				{
					app_metadata: {
						group_id: 'db78d6fe-b05e-476d-b908-cb6fff26a3ed',
						support_email: 'pelumi@mailinator.com',
						title: 'App A',
						uid: '41e3683f-2799-434d-ab61-4bfbe7c1ae23'
					},
					created_at: '2022-03-04T12:50:37.048Z',
					endpoint: {
						secret: 'kRfXPgJU6kAkc35H2-CqXwnrP_6wcEBVzA==',
						sent: false,
						status: 'active',
						target_url: 'https://webhook.site/ac06134f-b969-4388-b663-1e55951a99a4',
						uid: '8a069124-757e-4ad1-8939-6882a0f3e9bb'
					},
					event_metadata: {
						name: 'three',
						uid: '5bbca57e-e9df-4668-9208-827b962dc9a1'
					},
					metadata: {
						interval_seconds: 65,
						next_send_time: '2022-04-22T15:11:16.76Z',
						num_trials: 5,
						retry_limit: 5,
						strategy: 'default'
					},
					status: 'Failure',
					uid: 'b51ebc56-10df-42f1-8e00-6fb9da957bc0',
					updated_at: '2022-04-22T15:10:11.761Z'
				}
			]
		}
	];
	eventDeliveries!: { pagination: PAGINATION; content: EVENT_DELIVERY[] };
	sidebarEventDeliveries!: EVENT_DELIVERY[];
	eventDeliveryAtempt!: EVENT_DELIVERY_ATTEMPT;
	eventDeliveryFilteredByStatus: string[] = [];
	eventDelsTimeFilterData: { startTime: string; endTime: string } = { startTime: 'T00:00:00', endTime: 'T23:59:59' };
	eventsDelAppsFilter$!: Observable<APP[]>;
	filteredApps!: APP[];
	@ViewChild('eventDelsAppsFilter', { static: true }) eventDelsAppsFilter!: ElementRef;
	@ViewChild('eventDeliveryTimerFilter', { static: true }) eventDeliveryTimerFilter!: TimeFilterComponent;

	constructor(
		private formBuilder: FormBuilder,
		private generalService: GeneralService,
		private eventsService: EventsService,
		private datePipe: DatePipe,
		private route: ActivatedRoute,
		private router: Router
	) {}
	ngAfterViewInit() {
		this.eventsDelAppsFilter$ = fromEvent<any>(this.eventDelsAppsFilter?.nativeElement, 'keyup').pipe(
			map(event => event.target.value),
			startWith(''),
			debounceTime(500),
			distinctUntilChanged(),
			switchMap(search => this.getAppsForFilter(search))
		);
	}

	ngOnInit() {
		this.getEventDeliveries();
	}

	async getEventDeliveries(requestDetails?: { page?: number; addToURL?: boolean; fromFilter?: boolean }): Promise<HTTP_RESPONSE> {
		const page = requestDetails?.page || this.route.snapshot.queryParams.page || 1;
		if (requestDetails?.addToURL) this.addFilterToURL();
		const { startDate, endDate } = this.setDateForFilter({ ...this.eventDeliveriesFilterDateRange.value, ...this.eventDelsTimeFilterData });

		try {
			const eventDeliveriesResponse = await this.eventDeliveriesRequest({ pageNo: page, eventId: this.eventDeliveryFilteredByEventId, startDate, endDate });
			this.eventDeliveries = eventDeliveriesResponse.data;
			this.displayedEventDeliveries = this.generalService.setContentDisplayed(eventDeliveriesResponse.data.content);

			// if this is a filter request, set the eventDelsDetailsItem to the first item in the list
			if (requestDetails?.fromFilter) {
				this.eventDelsDetailsItem = this.eventDeliveries?.content[0];
				this.getDelieveryAttempts(this.eventDelsDetailsItem.uid);
			}

			this.isloadingEventDeliveries = false;
			return eventDeliveriesResponse;
		} catch (error: any) {
			this.isloadingEventDeliveries = false;
			return error;
		}
	}

	async eventDeliveriesRequest(requestDetails: { pageNo?: number; eventId?: string; startDate?: string; endDate?: string }): Promise<HTTP_RESPONSE> {
		let eventDeliveryStatusFilterQuery = '';
		this.eventDeliveryFilteredByStatus.length > 0 ? (this.eventDeliveriesStatusFilterActive = true) : (this.eventDeliveriesStatusFilterActive = false);
		this.eventDeliveryFilteredByStatus.forEach((status: string) => (eventDeliveryStatusFilterQuery += `&status=${status}`));

		try {
			const eventDeliveriesResponse = await this.eventsService.getEventDeliveries({
				eventId: requestDetails.eventId || '',
				pageNo: requestDetails.pageNo || 1,
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

	addFilterToURL() {
		const currentURLfilters = this.route.snapshot.queryParams;
		const queryParams: any = {};
		console.log(queryParams);
		const { startDate, endDate } = this.setDateForFilter({ ...this.eventDeliveriesFilterDateRange.value, ...this.eventDelsTimeFilterData });
		if (startDate) queryParams.eventDelsStartDate = startDate;
		if (endDate) queryParams.eventDelsEndDate = endDate;
		if (this.eventDeliveriesApp) queryParams.eventDelsApp = this.eventDeliveriesApp;
		queryParams.eventDelsStatus = this.eventDeliveryFilteredByStatus.length > 0 ? JSON.stringify(this.eventDeliveryFilteredByStatus) : '';

		this.router.navigate([], { queryParams: Object.assign({}, currentURLfilters, queryParams) });
	}

	setDateForFilter(requestDetails: { startDate: Date; endDate: Date; startTime?: string; endTime?: string }) {
		if (!requestDetails.endDate && !requestDetails.startDate) return { startDate: '', endDate: '' };
		const startDate = requestDetails.startDate ? `${format(requestDetails.startDate, 'yyyy-MM-dd')}${requestDetails?.startTime || 'T00:00:00'}` : '';
		const endDate = requestDetails.endDate ? `${format(requestDetails.endDate, 'yyyy-MM-dd')}${requestDetails?.endTime || 'T23:59:59'}` : '';
		return { startDate, endDate };
	}

	checkIfEventDeliveryStatusFilterOptionIsSelected(status: string): boolean {
		return this.eventDeliveryFilteredByStatus?.length > 0 ? this.eventDeliveryFilteredByStatus.includes(status) : false;
	}

	checkIfEventDeliveryAppFilterOptionIsSelected(appId: string): boolean {
		return appId === this.eventDeliveriesApp;
	}

	updateEventDevliveryStatusFilter(status: string, isChecked: any) {
		if (isChecked.target.checked) {
			this.eventDeliveryFilteredByStatus.push(status);
		} else {
			let index = this.eventDeliveryFilteredByStatus.findIndex(x => x === status);
			this.eventDeliveryFilteredByStatus.splice(index, 1);
		}
	}

	getSelectedDateRange(dateRange: { startDate: Date; endDate: Date }) {
		this.eventDeliveriesFilterDateRange.patchValue({
			startDate: dateRange.startDate,
			endDate: dateRange.endDate
		});
		this.getEventDeliveries({ addToURL: true });
	}

	clearFilters(filterType?: 'eventsDelApp' | 'eventsDelDate' | 'eventsDelsStatus') {
		const activeFilters = Object.assign({}, this.route.snapshot.queryParams);
		let filterItems: string[] = [];
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
		this.getEventDeliveries({ fromFilter: true });
	}

	fetchRetryCount() {}

	async getAppsForFilter(search: string): Promise<APP[]> {
		return await (
			await this.eventsService.getApps({ pageNo: 1, searchString: search })
		).data.content;
	}

	updateAppFilter(appId: string, isChecked: any) {
		this.showOverlay = false;
		this.showEventDeliveriesAppsDropdown = !this.showEventDeliveriesAppsDropdown;
		isChecked.target.checked ? (this.eventDeliveriesApp = appId) : (this.eventDeliveriesApp = '');

		this.getEventDeliveries({ addToURL: true, fromFilter: true });
	}

	formatDate(date: Date) {
		return this.datePipe.transform(date, 'dd/MM/yyyy');
	}

	async getDelieveryAttempts(eventDeliveryId: string) {
		this.isloadingDeliveryAttempts = true;
		try {
			const deliveryAttemptsResponse = await this.eventsService.getEventDeliveryAttempts({ eventDeliveryId });
			this.eventDeliveryAtempt = deliveryAttemptsResponse.data[deliveryAttemptsResponse.data.length - 1];
			this.isloadingDeliveryAttempts = false;

			return;
		} catch (error) {
			this.isloadingDeliveryAttempts = false;
			return error;
		}
	}

	async retryEvent(requestDetails: { e: any; index: number; eventDeliveryId: string }) {
		requestDetails.e.stopPropagation();
		const retryButton: any = document.querySelector(`#event${requestDetails.index} button`);
		if (retryButton) {
			retryButton.classList.add(['spin', 'disabled']);
			retryButton.disabled = true;
		}
		try {
			await this.eventsService.retryEvent({ eventId: requestDetails.eventDeliveryId });
			this.generalService.showNotification({ message: 'Retry Request Sent', style: 'success' });
			retryButton.classList.remove(['spin', 'disabled']);
			retryButton.disabled = false;
			this.getEventDeliveries();
		} catch (error: any) {
			this.generalService.showNotification({ message: `${error?.error?.message ? error?.error?.message : 'An error occured'}`, style: 'error' });
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
			await this.eventsService.forceRetryEvent({ body: payload });
			this.generalService.showNotification({ message: 'Force Retry Request Sent', style: 'success' });
			retryButton.classList.remove(['spin', 'disabled']);
			retryButton.disabled = false;
			this.getEventDeliveries();
		} catch (error: any) {
			this.generalService.showNotification({ message: `${error?.error?.message ? error?.error?.message : 'An error occured'}`, style: 'error' });
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
		this.isRetrying = true;
		try {
			const response = await this.eventsService.batchRetryEvent({
				eventId: this.eventDeliveryFilteredByEventId || '',
				pageNo: this.eventDeliveriesPage || 1,
				startDate: startDate,
				endDate: endDate,
				appId: this.eventDeliveriesApp,
				statusQuery: eventDeliveryStatusFilterQuery || ''
			});
			this.generalService.showNotification({ message: response.message, style: 'success' });
			this.getEventDeliveries();
			this.showBatchRetryModal = false;
			this.isRetrying = false;
		} catch (error: any) {
			this.isRetrying = false;
			this.generalService.showNotification({ message: error?.error?.message, style: 'error' });
			return error;
		}
	}
}
