import { Component, ElementRef, EventEmitter, OnInit, OnDestroy, Output, ViewChild } from '@angular/core';
import { ActivatedRoute } from '@angular/router';
import { EVENT_DELIVERY, FILTER_QUERY_PARAM } from 'src/app/models/event.model';
import { CURSOR, PAGINATION } from 'src/app/models/global.model';
import { HTTP_RESPONSE } from 'src/app/models/global.model';
import { GeneralService } from 'src/app/services/general/general.service';
import { EventsService } from '../events.service';
import { PrivateService } from 'src/app/private/private.service';
import { ProjectService } from '../../project.service';

@Component({
	selector: 'app-event-deliveries',
	templateUrl: './event-deliveries.component.html',
	styleUrls: ['./event-deliveries.component.scss']
})
export class EventDeliveriesComponent implements OnInit, OnDestroy {
	@Output() pushEventDeliveries = new EventEmitter<any>();
	eventDeliveryStatuses = ['Success', 'Failure', 'Retry', 'Scheduled', 'Processing', 'Discarded'];
	eventDelTableHead: string[] = ['Status', 'Event type', this.projectService.activeProjectDetails?.type == 'incoming' ? 'Subscription' : 'Endpoint', 'Attempts', 'Next Attempt', 'Time', '', ''];
	fetchingCount = false;
	showBatchRetryModal = false;
	isloadingEventDeliveries = false;
	isRetrying = false;
	batchRetryCount!: number;
	displayedEventDeliveries!: { date: string; content: any[] }[];
	eventDeliveries!: { pagination: PAGINATION; content: EVENT_DELIVERY[] };
	@ViewChild('batchRetryDialog', { static: true }) dialog!: ElementRef<HTMLDialogElement>;
	portalToken = this.route.snapshot.queryParams?.token;
	queryParams?: FILTER_QUERY_PARAM;
	getEventDeliveriesInterval: any;

	constructor(private generalService: GeneralService, private eventsService: EventsService, public route: ActivatedRoute, public projectService: ProjectService, public privateService: PrivateService) {}

	ngOnInit() {}

	ngOnDestroy() {
		clearInterval(this.getEventDeliveriesInterval);
	}

	fetchEventDeliveries(requestDetails?: FILTER_QUERY_PARAM) {
		const data = requestDetails;
		this.queryParams = data;
		this.getEventDeliveries({ ...data, showLoader: true });
	}

	checkIfTailModeIsEnabled() {
		const tailModeConfig = localStorage.getItem('EVENTS_TAIL_MODE');

		return tailModeConfig ? JSON.parse(tailModeConfig) : false;
	}

	handleTailing(tailDetails: { data: FILTER_QUERY_PARAM; tailModeConfig: boolean }) {
		this.queryParams = tailDetails.data;

		clearInterval(this.getEventDeliveriesInterval);
		if (tailDetails.tailModeConfig) this.getEventDeliveriesAtInterval(tailDetails.data);
	}

	getEventDeliveriesAtInterval(data: FILTER_QUERY_PARAM) {
		this.getEventDeliveriesInterval = setInterval(() => {
			this.getEventDeliveries(data);
		}, 5000);
	}

	async getEventDeliveries(requestDetails?: FILTER_QUERY_PARAM): Promise<HTTP_RESPONSE> {
		if (requestDetails?.showLoader) this.isloadingEventDeliveries = true;

		try {
			const eventDeliveriesResponse = await this.eventDeliveriesRequest(requestDetails);
			this.eventDeliveries = eventDeliveriesResponse.data;

			this.displayedEventDeliveries = this.setEventDeliveriesContent(eventDeliveriesResponse.data.content);

			this.isloadingEventDeliveries = false;
			return eventDeliveriesResponse;
		} catch (error: any) {
			this.isloadingEventDeliveries = false;
			return error;
		}
	}

	setEventDeliveriesContent(eventDeliveriesData: any[]) {
		const eventIds: any = [];
		const finalEventDels: any = [];
		let filteredEventDeliveries: any = [];

		const filteredEventDeliveriesByDate = this.generalService.setContentDisplayed(eventDeliveriesData, this.queryParams?.sort || 'desc');

		eventDeliveriesData.forEach((item: any) => {
			eventIds.push(item.event_id);
		});
		const uniqueEventIds = [...new Set(eventIds)];

		filteredEventDeliveriesByDate.forEach((eventDelivery: any) => {
			uniqueEventIds.forEach(eventId => {
				const filteredDeliveriesByEventId = eventDelivery.content.filter((item: any) => item.event_id === eventId);
				filteredEventDeliveries.push({ date: eventDelivery.date, event_id: eventId, eventDeliveries: filteredDeliveriesByEventId });
			});

			filteredEventDeliveries = filteredEventDeliveries.filter((item: any) => item.eventDeliveries.length !== 0);
			const uniqueEventDels = filteredEventDeliveries.filter((eventDels: any) => eventDelivery.date === eventDels.date);
			finalEventDels.push({ date: eventDelivery.date, content: uniqueEventDels });
		});

		return finalEventDels;
	}

	async eventDeliveriesRequest(requestDetails?: FILTER_QUERY_PARAM): Promise<HTTP_RESPONSE> {
		try {
			const eventDeliveriesResponse = await this.eventsService.getEventDeliveries(requestDetails);
			return eventDeliveriesResponse;
		} catch (error: any) {
			return error;
		}
	}

	async fetchRetryCount(data: FILTER_QUERY_PARAM) {
		this.queryParams = data;

		if (!data) return;

		this.fetchingCount = true;
		try {
			const response = await this.eventsService.getRetryCount(data);

			this.batchRetryCount = response.data.num;
			this.fetchingCount = false;
			this.dialog.nativeElement.showModal();
		} catch (error) {
			this.fetchingCount = false;
		}
	}

	paginateEvents(event: CURSOR) {
		this.queryParams = this.generalService.addFilterToURL({ ...this.queryParams, ...event });
		this.handleTailing({ data: this.queryParams, tailModeConfig: this.checkIfTailModeIsEnabled() });
		this.getEventDeliveries({ ...this.queryParams, showLoader: true });
	}

	async retryEvent(requestDetails: { e: any; index: number; eventDeliveryId: string }) {
		requestDetails.e.stopPropagation();

		try {
			const response = await this.eventsService.retryEvent({ eventId: requestDetails.eventDeliveryId });
			this.generalService.showNotification({ message: response.message, style: 'success' });
			return;
		} catch (error) {
			return error;
		}
	}

	// force retry successful events
	async forceRetryEvent(requestDetails: { e: any; index: number; eventDeliveryId: string }) {
		requestDetails.e.stopPropagation();
		const payload = {
			ids: [requestDetails.eventDeliveryId]
		};

		try {
			const response = await this.eventsService.forceRetryEvent({ body: payload });
			this.generalService.showNotification({ message: response.message, style: 'success' });
			return;
		} catch (error) {
			return error;
		}
	}

	async batchRetryEvent() {
		if (!this.queryParams) return;
		this.isRetrying = true;

		try {
			const response = await this.eventsService.batchRetryEvent(this.queryParams);

			this.generalService.showNotification({ message: response.message, style: 'success' });
			this.dialog.nativeElement.close();
			this.isRetrying = false;
			return;
		} catch (error) {
			this.isRetrying = false;
			return error;
		}
	}
}
