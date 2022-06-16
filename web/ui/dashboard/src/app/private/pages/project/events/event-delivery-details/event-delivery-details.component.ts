import { Location } from '@angular/common';
import { Component, HostListener, OnInit } from '@angular/core';
import { ActivatedRoute } from '@angular/router';
import { EVENT_DELIVERY_ATTEMPT } from 'src/app/models/event.model';
import { PrivateService } from 'src/app/private/private.service';
import { GeneralService } from 'src/app/services/general/general.service';
import { EventsService } from '../events.service';

@Component({
	selector: 'app-event-delivery-details',
	templateUrl: './event-delivery-details.component.html',
	styleUrls: ['./event-delivery-details.component.scss']
})
export class EventDeliveryDetailsComponent implements OnInit {
	isLoadingDeliveryDetails = false;
	isloadingDeliveryAttempts = false;
	shouldRenderSmallSize = false;
	eventDeliveryId = this.route.snapshot.params?.id;
	eventDelsDetailsItem: any;
	eventDeliveryAtempt!: EVENT_DELIVERY_ATTEMPT;
	screenWidth = window.innerWidth;
	constructor(private route: ActivatedRoute, private eventsService: EventsService, private generalService: GeneralService, private location: Location, public privateService: PrivateService) {}

	async ngOnInit() {
		this.isLoadingDeliveryDetails = true;
		await this.getEventDeliveryDetails();
	}

	async getEventDeliveryDetails() {
		this.isLoadingDeliveryDetails = true;

		try {
			const response = await this.eventsService.getDelivery(this.eventDeliveryId);
			this.eventDelsDetailsItem = response.data;
			this.getDeliveryAttempts({ eventDeliveryId: this.eventDelsDetailsItem.uid });
			this.isLoadingDeliveryDetails = false;
		} catch {
			this.isLoadingDeliveryDetails = false;
		}
	}

	async getDeliveryAttempts(requestDetails: { eventDeliveryId: string }) {
		this.isloadingDeliveryAttempts = true;
		try {
			const deliveryAttemptsResponse = await this.eventsService.getEventDeliveryAttempts({ eventDeliveryId: requestDetails.eventDeliveryId });
			this.eventDeliveryAtempt = deliveryAttemptsResponse.data[deliveryAttemptsResponse.data.length - 1];
			this.isloadingDeliveryAttempts = false;

			return;
		} catch (error) {
			this.isloadingDeliveryAttempts = false;
			return error;
		}
	}

	async forceRetryEvent(requestDetails: { e: any; eventDeliveryId: string }) {
		// requestDetails.e.stopPropagation();

		const payload = {
			ids: [requestDetails.eventDeliveryId]
		};
		try {
			await this.eventsService.forceRetryEvent({ body: payload });
			this.getEventDeliveryDetails();
			this.generalService.showNotification({ message: 'Force Retry Request Sent', style: 'success' });
		} catch (error: any) {
			this.generalService.showNotification({ message: `${error?.error?.message ? error?.error?.message : 'An error occured'}`, style: 'error' });
			return error;
		}
	}

	async retryEvent(requestDetails: { e: any; eventDeliveryId: string }) {
		// requestDetails.e.stopPropagation();

		try {
			await this.eventsService.retryEvent({ eventId: requestDetails.eventDeliveryId });
			this.getEventDeliveryDetails();
			this.generalService.showNotification({ message: 'Retry Request Sent', style: 'success' });
		} catch (error: any) {
			this.generalService.showNotification({ message: `${error?.error?.message ? error?.error?.message : 'An error occured'}`, style: 'error' });
			return error;
		}
	}

	getCodeSnippetString(type: 'res_body' | 'event' | 'event_delivery' | 'res_head' | 'req' | 'error') {
		if (type === 'event_delivery') {
			if (!this.eventDelsDetailsItem?.metadata?.data) return 'No event data was sent';
			return JSON.stringify(this.eventDelsDetailsItem.metadata.data, null, 4).replaceAll(/"([^"]+)":/g, '$1:');
		} else if (type === 'res_body') {
			if (!this.eventDeliveryAtempt || !this.eventDeliveryAtempt.response_data) return 'No response body was sent';
			return this.eventDeliveryAtempt.response_data;
		} else if (type === 'res_head') {
			if (!this.eventDeliveryAtempt || !this.eventDeliveryAtempt.response_http_header) return 'No response header was sent';
			return JSON.stringify(this.eventDeliveryAtempt.response_http_header, null, 4).replaceAll(/"([^"]+)":/g, '$1:');
		} else if (type === 'req') {
			if (!this.eventDeliveryAtempt || !this.eventDeliveryAtempt.request_http_header) return 'No request header was sent';
			return JSON.stringify(this.eventDeliveryAtempt.request_http_header, null, 4).replaceAll(/"([^"]+)":/g, '$1:');
		} else if (type === 'error') {
			if (this.eventDeliveryAtempt?.error) return JSON.stringify(this.eventDeliveryAtempt.error, null, 4).replaceAll(/"([^"]+)":/g, '$1:');
			return '';
		}
		return '';
	}

	goBack() {
		this.location.back();
	}

	checkScreenSize() {
		this.screenWidth > 1010 ? (this.shouldRenderSmallSize = false) : (this.shouldRenderSmallSize = true);
	}

	@HostListener('window:resize', ['$event'])
	onWindowResize() {
		this.screenWidth = window.innerWidth;
		this.checkScreenSize();
	}
}
