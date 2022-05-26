import { Component, OnInit } from '@angular/core';
import { EVENT_DELIVERY_ATTEMPT } from 'src/app/models/event.model';

@Component({
	selector: 'app-event-delivery-details',
	templateUrl: './event-delivery-details.component.html',
	styleUrls: ['./event-delivery-details.component.scss']
})
export class EventDeliveryDetailsComponent implements OnInit {
	eventDelsDetailsItem: any = {
		app_metadata: {
			group_id: 'db78d6fe-b05e-476d-b908-cb6fff26a3ed',
			support_email: 'pelumi@mailinator.com',
			title: 'App A',
			uid: '41e3683f-2799-434d-ab61-4bfbe7c1ae23'
		},
		created_at: '2022-03-04T12:50:37.048Z',
		description: 'Retry limit exceeded',
		endpoint: {
			http_timeout: '',
			rate_limit: 0,
			rate_limit_duration: '',
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
	};
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
	constructor() {}

	ngOnInit(): void {}

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
}
