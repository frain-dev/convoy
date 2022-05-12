import { ChangeDetectionStrategy, Component, Input, Output, EventEmitter, OnInit } from '@angular/core';

@Component({
	selector: 'event-delivery-details',
	changeDetection: ChangeDetectionStrategy.OnPush,
	templateUrl: './event-delivery-details.component.html',
	styleUrls: ['../../convoy-dashboard.component.scss']
})
export class EventDeliveryDetailsComponent implements OnInit {
	constructor() {}
	@Input('eventDelsDetailsItem') eventDelsDetailsItem!: any;
	@Input('eventDeliveryAtempt') eventDeliveryAtempt!: any;
	@Input('eventDeliveryIndex') eventDeliveryIndex!: any;
	@Output() loadEventsFromAppTable = new EventEmitter<string>();
	@Output() closeOverviewRender = new EventEmitter<any>();
	@Output() doForceRetryEvent = new EventEmitter<any>();
	@Output() doRetryEvent = new EventEmitter<any>();

	showPublicCopyText: boolean = true;
	showSecretCopyText: boolean = false;
	async ngOnInit() {}

	// close app or event deliveries overview
	closeOverview() {
		this.closeOverviewRender.emit();
	}

	// force retry event
	forceRetryEvent(requestDetails: any) {
		this.doForceRetryEvent.emit(requestDetails);
	}

	//  retry event
	retryEvent(requestDetails: any) {
		this.doRetryEvent.emit(requestDetails);
	}

	// get code snippet for prism
	getCodeSnippetString(type: 'res_body' | 'event' | 'event_delivery' | 'res_head' | 'req' | 'error') {
		if (type === 'event_delivery') {
			if (!this.eventDelsDetailsItem?.metadata?.data) return 'No event data was sent';
			return JSON.stringify(this.eventDelsDetailsItem.metadata.data, null, 4).replaceAll(/"([^"]+)":/g, '$1:');
		} else if (type === 'res_body') {
			if (!this.eventDeliveryAtempt || !this.eventDeliveryAtempt.response_data) return 'No response body was sent';
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
}
