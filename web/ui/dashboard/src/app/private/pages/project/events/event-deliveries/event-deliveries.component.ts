import { DatePipe } from '@angular/common';
import { Component, ElementRef, Input, OnInit, ViewChild } from '@angular/core';
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
	styleUrls: ['./event-deliveries.component.scss']
})
export class EventDeliveriesComponent implements OnInit {
	@Input() eventDeliveryFilteredByEventId?: string;
	dateOptions = ['Last Year', 'Last Month', 'Last Week', 'Yesterday'];
	eventDeliveryStatuses = ['Success', 'Failure', 'Retry', 'Scheduled', 'Processing', 'Discarded'];
	eventDelTableHead: string[] = ['Status', 'Event Type', 'Attempts', 'Max Attempts', 'Time Created', '', ''];
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
	dateFiltersFromURL: { startDate: string | Date; endDate: string | Date } = { startDate: '', endDate: '' };
	batchRetryCount!: number;
	eventDeliveriesApp?: string;
	eventDelsDetailsItem?: any;
	eventDeliveryIndex!: number;
	eventDeliveriesPage: number = 1;
	selectedEventsFromEventDeliveriesTable: string[] = [];
	displayedEventDeliveries!: { date: string; content: EVENT_DELIVERY[] }[];
	eventDeliveries!: { pagination: PAGINATION; content: EVENT_DELIVERY[] };
	sidebarEventDeliveries!: EVENT_DELIVERY[];
	eventDeliveryAtempt!: EVENT_DELIVERY_ATTEMPT;
	eventDeliveryFilteredByStatus: string[] = [];
	eventDelsTimeFilterData: { startTime: string; endTime: string } = { startTime: 'T00:00:00', endTime: 'T23:59:59' };
	eventsDelAppsFilter$!: Observable<APP[]>;
	filteredApps!: APP[];
	@ViewChild('eventDelsAppsFilter', { static: true }) eventDelsAppsFilter!: ElementRef;
	@ViewChild('eventDeliveryTimerFilter', { static: true }) eventDeliveryTimerFilter!: TimeFilterComponent;

	constructor(private generalService: GeneralService, private eventsService: EventsService, private datePipe: DatePipe, private route: ActivatedRoute, private router: Router) {}

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
		this.getFiltersFromURL();
		this.getEventDeliveries();
	}

	getFiltersFromURL() {
		const filters = this.route.snapshot.queryParams;
		if (Object.keys(filters).length == 0) return;

		this.dateFiltersFromURL = {
			startDate: filters.eventDelsStartDate ? new Date(filters.eventDelsStartDate) : '',
			endDate: filters.eventDelsEndDate ? new Date(filters.eventDelsEndDate) : ''
		};
		this.eventDeliveriesApp = filters.eventDelsApp ?? '';
		this.eventDeliveryFilteredByStatus = filters.eventDelsStatus ? JSON.parse(filters.eventDelsStatus) : [];
	}

	async getEventDeliveries(requestDetails?: { page?: number; addToURL?: boolean; fromFilter?: boolean }): Promise<HTTP_RESPONSE> {
		const page = requestDetails?.page || this.route.snapshot.queryParams.page || 1;
		if (requestDetails?.addToURL) this.addFilterToURL();
		const { startDate, endDate } = this.setDateForFilter({ ...this.dateFiltersFromURL, ...this.eventDelsTimeFilterData });
		this.isloadingEventDeliveries = true;

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
				appId: this.eventDeliveriesApp || '',
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
		const { startDate, endDate } = this.setDateForFilter({ ...this.dateFiltersFromURL, ...this.eventDelsTimeFilterData });
		if (startDate) queryParams.eventDelsStartDate = startDate;
		if (endDate) queryParams.eventDelsEndDate = endDate;
		if (this.eventDeliveriesApp) queryParams.eventDelsApp = this.eventDeliveriesApp;
		queryParams.eventDelsStatus = this.eventDeliveryFilteredByStatus.length > 0 ? JSON.stringify(this.eventDeliveryFilteredByStatus) : '';

		this.router.navigate([], { queryParams: Object.assign({}, currentURLfilters, queryParams) });
	}

	setDateForFilter(requestDetails: { startDate: any; endDate: any; startTime?: string; endTime?: string }) {
		if (!requestDetails.endDate && !requestDetails.startDate) return { startDate: '', endDate: '' };
		const startDate = requestDetails.startDate ? `${format(requestDetails.startDate, 'yyyy-MM-dd')}${requestDetails?.startTime || 'T00:00:00'}` : '';
		const endDate = requestDetails.endDate ? `${format(requestDetails.endDate, 'yyyy-MM-dd')}${requestDetails?.endTime || 'T23:59:59'}` : '';
		return { startDate, endDate };
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

	getSelectedDateRange(dateRange: { startDate: Date; endDate: Date }) {
		this.dateFiltersFromURL = { ...dateRange };
		this.getEventDeliveries({ addToURL: true });
	}

	clearFilters(filterType?: 'eventsDelApp' | 'eventsDelDate' | 'eventsDelsStatus') {
		const activeFilters = Object.assign({}, this.route.snapshot.queryParams);
		let filterItems: string[] = [];
		this.eventDeliveriesApp = undefined;

		switch (filterType) {
			case 'eventsDelApp':
				this.eventDeliveryFilteredByEventId = undefined;
				filterItems = ['eventDelsApp'];
				break;
			case 'eventsDelDate':
				this.eventDelsTimeFilterData = { startTime: 'T00:00:00', endTime: 'T23:59:59' };
				this.dateFiltersFromURL = { startDate: '', endDate: '' };
				filterItems = ['eventDelsStartDate', 'eventDelsEndDate'];
				break;
			case 'eventsDelsStatus':
				this.eventDeliveryFilteredByStatus = [];
				filterItems = ['eventDelsStatus'];
				break;
			default:
				this.dateFiltersFromURL = { startDate: '', endDate: '' };
				this.eventDeliveryFilteredByEventId = undefined;
				this.eventDeliveryFilteredByStatus = [];
				this.eventDelsTimeFilterData = { startTime: 'T00:00:00', endTime: 'T23:59:59' };
				filterItems = ['eventDelsStartDate', 'eventDelsEndDate', 'eventDelsApp', 'eventDelsStatus'];
				break;
		}

		filterItems.forEach(key => (activeFilters.hasOwnProperty(key) ? delete activeFilters[key] : null));
		this.router.navigate([], { relativeTo: this.route, queryParams: activeFilters });
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
		isChecked.target.checked ? (this.eventDeliveriesApp = appId) : (this.eventDeliveriesApp = undefined);

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
		const { startDate, endDate } = this.setDateForFilter(this.dateFiltersFromURL);
		this.isRetrying = true;

		try {
			const response = await this.eventsService.batchRetryEvent({
				eventId: this.eventDeliveryFilteredByEventId || '',
				pageNo: this.eventDeliveriesPage || 1,
				startDate: startDate,
				endDate: endDate,
				appId: this.eventDeliveriesApp || '',
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
