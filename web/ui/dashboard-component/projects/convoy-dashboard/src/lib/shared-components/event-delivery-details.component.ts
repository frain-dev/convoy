import { ChangeDetectionStrategy, Component, Input, Output, EventEmitter, OnInit } from '@angular/core';

@Component({
	selector: 'event-delivery-details',
	changeDetection: ChangeDetectionStrategy.OnPush,
	template: `
		<div class="page page__small padding-top__28px margin-bottom__20px" *ngIf="eventDelsDetailsItem">
			<div class="flex flex__justify-between flex__align-items-center margin-bottom__24px">
				<h3>Overview</h3>
				<button
					[disabled]="eventDelsDetailsItem?.status !== 'Failure' && eventDelsDetailsItem?.status !== 'Success'"
					[class]="'button__retry button--has-icon icon-left '"
					(click)="
						eventDelsDetailsItem.status === 'Success'
							? forceRetryEvent({ e: $event, index: this.eventDeliveryIndex, eventDeliveryId: eventDelsDetailsItem.uid })
							: retryEvent({ e: $event, index: this.eventDeliveryIndex, eventDeliveryId: eventDelsDetailsItem.uid })
					"
				>
					<img src="/assets/img/refresh-icon-primary.svg" alt="refresh icon" />

					{{ eventDelsDetailsItem.status === 'Success' ? 'Force Retry' : 'Retry' }}
				</button>
			</div>
			<div class="grid grid__col-5 margin-bottom__24px">
				<div>
					<p class="color__grey font__12px font__weight-400">EVENT TYPE</p>
					<p class="color__black font__14px font__weight-500">{{ eventDelsDetailsItem?.created_at | date: 'mediumDate' }}</p>
				</div>

				<div>
					<p class="color__grey font__12px font__weight-400">ATTEMPTS</p>
					<p class="color__black font__14px font__weight-500">{{ eventDelsDetailsItem?.metadata?.num_trials }}</p>
				</div>

				<div>
					<p class="color__grey font__12px font__weight-400">STATUS</p>
					<div [class]="'tag tag--' + eventDelsDetailsItem.status">{{ eventDelsDetailsItem.status }}</div>
				</div>
				<div>
					<p class="color__grey font__12px font__weight-400">IP ADDRESS</p>
					<p class="color__black font__14px font__weight-500">{{ eventDeliveryAtempt?.ip_address || '-' }}</p>
				</div>
				<div *ngIf="eventDelsDetailsItem.status == 'Success'">
					<p class="color__grey font__12px font__weight-400">TIME</p>
					<p class="color__black font__14px font__weight-500">{{ eventDelsDetailsItem?.updated_at | date: 'medium' }}</p>
				</div>
			</div>
			<div class="grid grid__col-5 margin-bottom__32px">
				<div>
					<p class="color__grey font__12px font__weight-400">HTTP STATUS</p>
					<p class="color__black font__14px font__weight-500">{{ eventDeliveryAtempt?.http_status || '-' }}</p>
				</div>
				<div>
					<p class="color__grey font__12px font__weight-400">API VERSION</p>
					<p class="color__black font__14px font__weight-500">{{ eventDeliveryAtempt?.api_version || '-' }}</p>
				</div>
				<div>
					<p class="color__grey font__12px font__weight-400">ENDPOINT</p>
					<p class="color__primary font__14px font__weight-500 long-text long-text__200px" [title]="eventDelsDetailsItem.endpoint?.target_url">{{ eventDelsDetailsItem.endpoint?.target_url }}</p>
				</div>
				<div></div>
				<div *ngIf="eventDelsDetailsItem.status === 'Success'"></div>
			</div>
			<div class="grid border__top grid__col-2">
				<div class="eventDelivery border__right padding-top__40px padding-right__32px padding-bottom__40px width__100">
					<h3 class="margin-bottom__16px color__black">Events</h3>
					<prism language="json" [code]="getCodeSnippetString('event_delivery')"></prism>
				</div>
				<div class="eventDelivery padding-left__32px padding-top__40px padding-bottom__40px width__100">
					<h3 class="margin-bottom__16px color__black">Response Header</h3>
					<prism language="json" [code]="getCodeSnippetString('res_head')"></prism>
				</div>
			</div>
			<div class="grid border__top grid__col-2">
				<ng-container *ngIf="eventDeliveryAtempt?.error">
					<div class="eventDelivery padding-top__40px padding-right__32px padding-bottom__40px width__100">
						<h3 class="margin-bottom__16px color__black">Error</h3>
						<prism language="json" [code]="getCodeSnippetString('error')"></prism>
					</div>
				</ng-container>
				<ng-container *ngIf="!eventDeliveryAtempt?.error">
					<div class="eventDelivery padding-top__40px padding-right__32px padding-bottom__40px width__100">
						<h3 class="margin-bottom__16px color__black">Response Body</h3>
						<prism language="json" [code]="getCodeSnippetString('res_body')"></prism>
					</div>
				</ng-container>
				<div class="eventDelivery padding-top__40px padding-left__32px padding-bottom__40px width__100">
					<h3 class="margin-bottom__16px color__black">Request Header</h3>
					<prism language="json" [code]="getCodeSnippetString('req')"></prism>
				</div>
			</div>
		</div>
	`,
	styleUrls: ['../convoy-dashboard.component.scss']
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
