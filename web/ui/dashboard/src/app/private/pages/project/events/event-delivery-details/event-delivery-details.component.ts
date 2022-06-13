import { Location } from '@angular/common';
import { Component, OnInit } from '@angular/core';
import { ActivatedRoute } from '@angular/router';
import { EVENT_DELIVERY_ATTEMPT } from 'src/app/models/event.model';
import { PrivateService } from 'src/app/private/private.service';
import { EventDeliveryDetailsService } from './event-delivery-details.service';

@Component({
	selector: 'app-event-delivery-details',
	templateUrl: './event-delivery-details.component.html',
	styleUrls: ['./event-delivery-details.component.scss']
})
export class EventDeliveryDetailsComponent implements OnInit {
	eventDelsDetails: any;
	eventDeliveryAtempt!: EVENT_DELIVERY_ATTEMPT;

	constructor(public privateService: PrivateService, private route: ActivatedRoute, private eventDeliveryDetailsService: EventDeliveryDetailsService, private location: Location) {}

	ngOnInit(): void {
		const eventDeliveryId = this.route.snapshot.params.id;
		this.getEventDeliveryDetails(eventDeliveryId);
		this.getEventDeliveryAttempts(eventDeliveryId);
	}

	goBack() {
		this.location.back();
	}

	async getEventDeliveryDetails(id: string) {
		try {
			const response = await this.eventDeliveryDetailsService.getEventDeliveryDetails(id);
			this.eventDelsDetails = response.data;
		} catch (error) {
			console.log(error);
		}
	}

	async getEventDeliveryAttempts(eventId: string) {
		try {
			const response = await this.eventDeliveryDetailsService.getEventDeliveryAttempts(eventId);
			this.eventDeliveryAtempt = response.data[response.data.length - 1];
		} catch (error) {
			console.log(error);
		}
	}

	getCodeSnippetString(type: 'res_body' | 'event' | 'event_delivery' | 'res_head' | 'req' | 'error') {
		if (type === 'event_delivery') {
			if (!this.eventDelsDetails?.metadata?.data) return 'No event data was sent';
			return JSON.stringify(this.eventDelsDetails.metadata.data, null, 4).replaceAll(/"([^"]+)":/g, '$1:');
		} else if (type === 'res_body') {
			if (!this.eventDeliveryAtempt || !this.eventDeliveryAtempt?.response_data) return 'No response body was sent';
			return this.eventDeliveryAtempt?.response_data;
		} else if (type === 'res_head') {
			if (!this.eventDeliveryAtempt || !this.eventDeliveryAtempt?.response_http_header) return 'No response header was sent';
			return JSON.stringify(this.eventDeliveryAtempt.response_http_header, null, 4).replaceAll(/"([^"]+)":/g, '$1:');
		} else if (type === 'req') {
			if (!this.eventDeliveryAtempt || !this.eventDeliveryAtempt?.request_http_header) return 'No request header was sent';
			return JSON.stringify(this.eventDeliveryAtempt.request_http_header, null, 4).replaceAll(/"([^"]+)":/g, '$1:');
		} else if (type === 'error') {
			if (this.eventDeliveryAtempt?.error) return JSON.stringify(this.eventDeliveryAtempt.error, null, 4).replaceAll(/"([^"]+)":/g, '$1:');
			return '';
		}
		return '';
	}
}
