import { Location } from '@angular/common';
import { Component, OnInit } from '@angular/core';
import { ActivatedRoute } from '@angular/router';
import { EVENT_DELIVERY_ATTEMPT } from 'src/app/models/event.model';
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
	eventDelsDetailsItem: any;
	eventDeliveryAtempt: EVENT_DELIVERY_ATTEMPT = {
		api_version: '2021-08-27',
		created_at: '2022-03-04T12:50:38.958Z',
		http_status: '200 OK',
		ip_address: '46.4.105.116:443',
		request_http_header: {
			'Content-Type': 'application/json',
			'User-Agent': 'Convoy/v0.4.14',
			'X-Project-Signature-Y': '265557169ef78545c75bfdc5c751f44765bd4cc16315bc1c3411413a'
		},
		response_data: '{"success":false,"error":{"message":"Token not found","id":null}}',
		response_http_header: {
			'Cache-Control': 'no-cache, private',
			'Content-Type': 'text/plain; charset=UTF-8',
			Date: 'Fri, 04 Mar 2022 12:50:38 GMT',
			Server: 'nginx',
			Vary: 'Accept-Encoding',
			'X-Request-Id': '03fbb49b-80a1-4bd6-bde2-32ddf30b80f9',
			'X-Token-Id': 'ac06134f-b969-4388-b663-1e55951a99a4'
		}
	};
	constructor(private route: ActivatedRoute, private eventsService: EventsService, private generalService: GeneralService, private location: Location) {}

	ngOnInit() {
		this.getDeliveryId();
	}

	getDeliveryId() {
		this.route.params.subscribe(res => {
			const deliveryId = res.id;
			this.getEventDeliveryDetails(deliveryId);
		});
	}

	async getEventDeliveryDetails(deliveryId: string) {
		this.isLoadingDeliveryDetails = true;

		try {
			const response = await this.eventsService.getDelivery(deliveryId);
			this.eventDelsDetailsItem = response.data;
			this.getDeliveryAttempts({ eventId: this.eventDelsDetailsItem.event_id, eventDeliveryId: this.eventDelsDetailsItem.uid });
			this.isLoadingDeliveryDetails = false;
		} catch {
			this.isLoadingDeliveryDetails = false;
		}
	}

	async getDeliveryAttempts(requestDetails: { eventId: string; eventDeliveryId: string }) {
		this.isloadingDeliveryAttempts = true;
		try {
			const deliveryAttemptsResponse = await this.eventsService.getEventDeliveryAttempts({ eventId: requestDetails.eventId, eventDeliveryId: requestDetails.eventDeliveryId });
			this.eventDeliveryAtempt = deliveryAttemptsResponse.data[deliveryAttemptsResponse.data.length - 1];
			this.isloadingDeliveryAttempts = false;

			return;
		} catch (error) {
			this.isloadingDeliveryAttempts = false;
			return error;
		}
	}

	async forceRetryEvent(requestDetails: { e: any; index: number; eventDeliveryId: string }) {
		requestDetails.e.stopPropagation();

		const payload = {
			ids: [requestDetails.eventDeliveryId]
		};
		try {
			await this.eventsService.forceRetryEvent({ body: payload });
			this.generalService.showNotification({ message: 'Force Retry Request Sent', style: 'success' });

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

	goBack() {
		this.location.back();
	}
}
