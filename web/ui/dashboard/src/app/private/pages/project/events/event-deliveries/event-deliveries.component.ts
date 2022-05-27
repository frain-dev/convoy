import { DatePipe } from '@angular/common';
import { Component, ElementRef, OnInit, ViewChild } from '@angular/core';
import { FormBuilder, FormGroup } from '@angular/forms';
import { Observable } from 'rxjs';
import { APP } from 'src/app/models/app.model';
import { EVENT_DELIVERY, EVENT_DELIVERY_ATTEMPT } from 'src/app/models/event.model';
import { PAGINATION } from 'src/app/models/global.model';
import { GeneralService } from 'src/app/services/general/general.service';
import { EventsService } from '../events.service';

@Component({
	selector: 'app-event-deliveries',
	templateUrl: './event-deliveries.component.html',
	styleUrls: ['../events.component.scss']
})
export class EventDeliveriesComponent implements OnInit {
	dateOptions = ['Last Year', 'Last Month', 'Last Week', 'Yesterday'];
	eventDeliveryStatuses = ['Success', 'Failure', 'Retry', 'Scheduled', 'Processing', 'Discarded'];
	eventDelTableHead: string[] = ['Status', 'Event Type', 'Attempts', 'Time Created', '', ''];
	showEventDelFilterCalendar: boolean = false;
	showEventDeliveriesStatusDropdown: boolean = false;
	eventDeliveriesStatusFilterActive: boolean = false;
	showEventDeliveriesAppsDropdown: boolean = false;
	showOverlay: boolean = false;
	fetchingCount: boolean = false;
	isloadingEventDeliveries: boolean = false;
	isloadingDeliveryAttempts: boolean = false;
	eventDeliveriesFilterDateRange: FormGroup = this.formBuilder.group({
		startDate: [{ value: '', disabled: true }],
		endDate: [{ value: '', disabled: true }]
	});
	selectedEventsDelDateOption: string = '';
	eventDeliveriesApp!: string;
	activeProjectId!: string;
	eventDeliveryFilteredByEventId!: string;
	eventDelsDetailsItem?: any;
	eventDeliveryIndex!: number;
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
	constructor(private formBuilder: FormBuilder, private generalService: GeneralService, private eventsService: EventsService, private datePipe: DatePipe) {}

	ngOnInit(): void {}

	getEventDeliveries(requestDetails?: any) {}

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

	getSelectedDate(dateOption: string) {
		this.selectedEventsDelDateOption = dateOption;
		const { startDate, endDate } = this.generalService.getSelectedDate(dateOption);
		this.eventDeliveriesFilterDateRange.patchValue({
			startDate: startDate,
			endDate: endDate
		});
		this.getEventDeliveries({ addToURL: true, fromFilter: true });
	}

	clearFilters(filterType?: 'eventsDelDate' | 'eventsDelApp' | 'eventsDelsStatus') {}
	fetchRetryCount() {}
	async getAppsForFilter(search: string): Promise<APP[]> {
		return await (
			await this.eventsService.getApps({ activeProjectId: this.activeProjectId, pageNo: 1, searchString: search })
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
			const deliveryAttemptsResponse = await this.eventsService.getEventDeliveryAttempts({ eventDeliveryId, activeProjectId: this.activeProjectId });
			this.eventDeliveryAtempt = deliveryAttemptsResponse.data[deliveryAttemptsResponse.data.length - 1];
			this.isloadingDeliveryAttempts = false;

			return;
		} catch (error) {
			this.isloadingDeliveryAttempts = false;
			return error;
		}
	}

	async retryEvent(requestDetails: { e: any; index: number; eventDeliveryId: string }) {
		// requestDetails.e.stopPropagation();
		// const retryButton: any = document.querySelector(`#event${requestDetails.index} button`);
		// if (retryButton) {
		// 	retryButton.classList.add(['spin', 'disabled']);
		// 	retryButton.disabled = true;
		// }
		// try {
		// 	await this.eventsService.retryEvent({ eventId: requestDetails.eventDeliveryId });
		// 	this.eventsService.showNotification({ message: 'Retry Request Sent', style: 'success' });
		// 	retryButton.classList.remove(['spin', 'disabled']);
		// 	retryButton.disabled = false;
		// 	this.getEventDeliveries();
		// } catch (error: any) {
		// 	this.eventsService.showNotification({ message: `${error?.error?.message ? error?.error?.message : 'An error occured'}`, style: 'error' });
		// 	if (retryButton) {
		// 		retryButton.classList.remove(['spin', 'disabled']);
		// 		retryButton.disabled = false;
		// 	}
		// 	return error;
		// }
	}

	// force retry successful events
	async forceRetryEvent(requestDetails: { e: any; index: number; eventDeliveryId: string }) {
		// requestDetails.e.stopPropagation();
		// const retryButton: any = document.querySelector(`#event${requestDetails.index} button`);
		// if (retryButton) {
		// 	retryButton.classList.add(['spin', 'disabled']);
		// 	retryButton.disabled = true;
		// }
		// const payload = {
		// 	ids: [requestDetails.eventDeliveryId]
		// };
		// try {
		// 	await this.eventsService.forceRetryEvent({ body: payload });
		// 	this.eventsService.showNotification({ message: 'Force Retry Request Sent', style: 'success' });
		// 	retryButton.classList.remove(['spin', 'disabled']);
		// 	retryButton.disabled = false;
		// 	this.getEventDeliveries();
		// } catch (error: any) {
		// 	this.eventsService.showNotification({ message: `${error?.error?.message ? error?.error?.message : 'An error occured'}`, style: 'error' });
		// 	if (retryButton) {
		// 		retryButton.classList.remove(['spin', 'disabled']);
		// 		retryButton.disabled = false;
		// 	}
		// 	return error;
		// }
	}

	async batchRetryEvent() {
		// let eventDeliveryStatusFilterQuery = '';
		// this.eventDeliveryFilteredByStatus.length > 0 ? (this.eventDeliveriesStatusFilterActive = true) : (this.eventDeliveriesStatusFilterActive = false);
		// this.eventDeliveryFilteredByStatus.forEach((status: string) => (eventDeliveryStatusFilterQuery += `&status=${status}`));
		// const { startDate, endDate } = this.setDateForFilter(this.eventDeliveriesFilterDateRange.value);
		// this.isRetyring = true;
		// try {
		// 	const response = await this.eventsService.batchRetryEvent({
		// 		eventId: this.eventDeliveryFilteredByEventId || '',
		// 		pageNo: this.eventDeliveriesPage || 1,
		// 		startDate: startDate,
		// 		endDate: endDate,
		// 		appId: this.eventDeliveriesApp,
		// 		statusQuery: eventDeliveryStatusFilterQuery || ''
		// 	});
		// 	this.eventsService.showNotification({ message: response.message, style: 'success' });
		// 	this.getEventDeliveries();
		// 	this.showBatchRetryModal = false;
		// 	this.isRetyring = false;
		// } catch (error: any) {
		// 	this.isRetyring = false;
		// 	this.eventsService.showNotification({ message: error?.error?.message, style: 'error' });
		// 	return error;
		// }
	}
}
