import { ChangeDetectionStrategy, Component, Input, Output, EventEmitter, OnInit } from '@angular/core';

@Component({
	selector: 'convoy-overview',
	changeDetection: ChangeDetectionStrategy.OnPush,
	template: `
		<div class="page page__small without-background padding-all__0px margin-top__30px margin-bottom__16px">
			<div class="flex flex__align-items-center flex__justify-between">
				<div class="flex flex__align-items-center">
					<button class="button__back flex flex__align-items-center flex__justify-center" (click)="closeOverview()">
						<img src="/assets/img/arrow-left-primary.svg" alt="arrow back" />
					</button>
					<p class="margin-bottom__0px font__16px color__black font__weight-500 margin-left__18px text__capitalize">
						{{ activeTab === 'apps' ? appsDetailsItem.name : eventDelsDetailsItem?.app_metadata?.title }}
					</p>
				</div>
				<div *ngIf="activeTab === 'apps' && appsDetailsItem">
					<!-- app enable/disable toggle  -->
					<label class="toggle">
						<span class="toggle-label">{{ appsDetailsItem?.is_disabled ? 'App Disabled' : 'App Enabled' }}</span>
						<input class="toggle-checkbox" type="checkbox" (change)="editAppStatus(appsDetailsItem)" [checked]="!appsDetailsItem?.is_disabled" />
						<div class="toggle-switch"></div>
					</label>
				</div>
			</div>
		</div>

		<!-- app details  -->
		<div class="page page__small padding-top__28px margin-bottom__20px" *ngIf="activeTab === 'apps' && appsDetailsItem" [ngClass]="{ disabled: appsDetailsItem?.is_disabled }">
			<div class="flex flex__justify-between flex__align-items-center margin-bottom__24px">
				<h3>Overview</h3>
				<button class="button button__white button__small" (click)="loadEventsFromAppsTable(appsDetailsItem.uid); closeOverview()">View Event</button>
			</div>
			<div class="flex flex__align-items-center flex__justify-between margin-bottom__32px">
				<div>
					<p class="color__grey font__12px font__weight-400">DATE CREATED</p>
					<p class="color__black font__14px font__weight-500">{{ appsDetailsItem?.created_at | date: 'mediumDate' }}</p>
				</div>
				<div>
					<p class="color__grey font__12px font__weight-400">SUPPORT EMAIL</p>
					<p class="color__black font__14px font__weight-500">{{ appsDetailsItem?.support_email || '...no support email provided' }}</p>
				</div>
				<div>
					<p class="color__grey font__12px font__weight-400">UPDATED</p>
					<p class="color__black font__14px font__weight-500">{{ appsDetailsItem?.updated_at | date: 'mediumDate' }}</p>
				</div>
				<div>
					<p class="color__grey font__12px font__weight-400">EVENTS</p>
					<p class="color__black font__14px font__weight-500">{{ appsDetailsItem?.events }}</p>
				</div>
				<div>
					<p class="color__grey font__12px font__weight-400">ENDPOINTS</p>
					<p class="color__black font__14px font__weight-500">{{ appsDetailsItem?.endpoints.length }}</p>
				</div>
			</div>
			<!-- pending when backend is ready  -->
			<!-- <div>
				<p class="flex flex__align-items-center font__14px font__weight-500 margin-bottom__8px">
					Enable Notification
					<img src="/assets/img/primary-info-icon.svg" class="margin-left__10px" alt="info icon" />
				</p>
				<label class="toggle">
					<input class="toggle-checkbox" type="checkbox" (change)="editAppStatus(appsDetailsItem)" [checked]="!appsDetailsItem?.is_disabled" />
					<div class="toggle-switch"></div>
				</label>
			</div> -->

			<div class="flex flex__justify-between border__top margin-top__22px">
				<div class="border__right width__50 padding-right__32px padding-top__32px">
					<div class="flex flex__align-items-center flex__justify-between">
						<h3>App Event Endpoints</h3>
						<div class="flex flex__align-items-center">
							<button class="button__clear" (click)="addEndpointModal()">Add Endpoints</button>
							<div class="line margin-right__16px margin-left__16px border__left"></div>
							<button class="button__clear" [disabled]="appsDetailsItem?.endpoints.length == 0" (click)="setEventAppId()">Add Event</button>
						</div>
					</div>
					<ul class="margin-top__16px">
						<ng-container *ngIf="appsDetailsItem?.endpoints">
							<li class="dashboard--logs--details--endpoints bg__grey-fade padding-all__16px rounded__8px margin-bottom__24px" *ngFor="let endpoint of appsDetailsItem.endpoints">
								<div>
									<div class="flex">
										<h5 class="color__black font__14px font__weight-400">{{ endpoint.description }}</h5>
										<button class="margin-left__16px button__clear color__primary button--has-icon icon-right small-icon" (click)="viewEndpointSecretKey(endpoint.secret)">
											View Secret
											<img src="/assets/img/arrow-up-right.svg" alt="link out" />
										</button>
									</div>
									<p class="flex flex__align-items-center font__14px color__black font__weight-300 margin-top__16px">
										<img src="/assets/img/link-icon.svg" alt="link icon" class="margin-right__8px" />
										{{ endpoint.target_url }}
									</p>
									<div class="flex margin-top__16px">
										<div class="tag tag__events" *ngFor="let event of endpoint.events">{{ event == '*' ? 'all events' : event }}</div>
									</div>
								</div>
								<div class="dashboard--logs--details--endpoints--inactive" *ngIf="endpoint.status == 'inactive'">
									<div class="icon">
										<img src="/assets/img/lock.svg" alt="lock icon" />
									</div>
									<p class="color__dark font__16px font__weight-600">Endpoint Disabled</p>
									<!-- pending till this is figured out by backend -->
									<!-- <a class="color__primary font__14px font__weight-500 margin-bottom__10px">Click here to learn how to enable this endpoint</a> -->
								</div>
							</li>
						</ng-container>
					</ul>
					<div class="empty-state smaller-table table--container" *ngIf="appsDetailsItem?.endpoints?.length === 0">
						<img src="/assets/img/empty-state-img.svg" alt="empty state" />
						<p>No endpoint has been added for selected app yet</p>
					</div>
				</div>
				<div class="width__50 padding-left__32px padding-top__32px">
					<div>
						<h3>App Portal</h3>
						<ul class="dashboard--logs--details--meta">
							<li class="list-item-inline">
								<div class="list-item-inline--label">App Page</div>
								<div class="list-item-inline--item link" (click)="getAppPortalToken({ redirect: true })">
									Open Link
									<img src="/assets/img/arrow-up-right.svg" alt="link out" />
								</div>
							</li>
							<li class="list-item-inline">
								<div class="list-item-inline--label">Embed into your platform</div>
								<a class="list-item-inline--item link" href="https://getconvoy.io/docs/app-portal" target="_blank">
									See Documentation
									<img src="/assets/img/arrow-up-right.svg" alt="link out" />
								</a>
							</li>
							<li class="list-item-inline">
								<div class="list-item-inline--label">Embed Iframe portal</div>
								<div class="list-item-inline--item"></div>
							</li>
							<div class="code">
								<div class="text">{{ appPortalLink }}</div>
								<div class="flex flex__justify-end">
									<button class="button__clear button--has-icon icon-left" (click)="copyKey(appPortalLink, 'secret')">
										<img src="/assets/img/copy.svg" alt="copy" />
										<small *ngIf="showSecretCopyText">Copied!</small>
									</button>
								</div>
							</div>
						</ul>
					</div>
				</div>
			</div>
		</div>

		<!-- event delivery details  -->
		<div class="page page__small padding-top__28px margin-bottom__20px" *ngIf="activeTab === 'event deliveries' && eventDelsDetailsItem">
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
				<div class="border__right padding-top__40px padding-right__32px padding-bottom__40px width__100">
					<h3 class="margin-bottom__16px color__black">Events</h3>
					<prism language="json" [code]="getCodeSnippetString('event_delivery')"></prism>
				</div>
				<div class="padding-left__32px padding-top__40px padding-bottom__40px width__100">
					<h3 class="margin-bottom__16px color__black">Response Header</h3>
					<prism language="json" [code]="getCodeSnippetString('res_head')"></prism>
				</div>
			</div>
			<div class="grid border__top grid__col-2">
				<ng-container *ngIf="eventDeliveryAtempt?.error">
					<div class="padding-top__40px padding-right__32px padding-bottom__40px width__100">
						<h3 class="margin-bottom__16px color__black">>Error</h3>
						<prism language="json" [code]="getCodeSnippetString('error')"></prism>
					</div>
				</ng-container>
				<ng-container *ngIf="!eventDeliveryAtempt?.error">
					<div class="padding-top__40px padding-right__32px padding-bottom__40px width__100">
						<h3 class="margin-bottom__16px color__black">Response Body</h3>
						<prism language="json" [code]="getCodeSnippetString('res_body')"></prism>
					</div>
				</ng-container>
				<div class="padding-top__40px padding-left__32px padding-bottom__40px width__100">
					<h3 class="margin-bottom__16px color__black">Request Header</h3>
					<prism language="json" [code]="getCodeSnippetString('req')"></prism>
				</div>
			</div>
		</div>
	`,
	styleUrls: ['../convoy-dashboard.component.scss']
})
export class ConvoyOverviewComponent implements OnInit {
	constructor() {}
	@Input('activeTab') activeTab!: string;
	@Input('appsDetailsItem') appsDetailsItem!: any;
	@Input('eventDelsDetailsItem') eventDelsDetailsItem!: any;
	@Input('eventDeliveryAtempt') eventDeliveryAtempt!: any;
	@Input('eventDeliveryIndex') eventDeliveryIndex!: any;
	@Input('appPortalLink') appPortalLink!: string;
	@Output() loadEventsFromAppTable = new EventEmitter<string>();
	@Output() openAddEndpointModal = new EventEmitter<any>();
	@Output() closeOverviewRender = new EventEmitter<any>();
	@Output() openEndpointSecretKey = new EventEmitter<string>();
	@Output() editAppDetails = new EventEmitter<any>();
	@Output() fetchAppPortalToken = new EventEmitter<any>();
	@Output() openAddEventModal = new EventEmitter<any>();
	@Output() doForceRetryEvent = new EventEmitter<any>();
	@Output() doRetryEvent = new EventEmitter<any>();

	showPublicCopyText: boolean = true;
	showSecretCopyText: boolean = false;
	async ngOnInit() {}

	setEventAppId() {
		this.openAddEventModal.emit();
	}

	getAppPortalToken(requestDetails: any) {
		this.fetchAppPortalToken.emit(requestDetails);
	}

	// load events from events table
	loadEventsFromAppsTable(appUid: string) {
		this.loadEventsFromAppTable.emit(appUid);
	}

	// edit app status
	editAppStatus(appsDetailsItem: any) {
		this.editAppDetails.emit(appsDetailsItem);
	}

	//open add new endpoint modal
	addEndpointModal() {
		this.openAddEndpointModal.emit();
	}

	// close app or event deliveries overview
	closeOverview() {
		this.closeOverviewRender.emit();
	}

	// view endpoint secret
	viewEndpointSecretKey(secretKey: string) {
		this.openEndpointSecretKey.emit(secretKey);
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

	// copy code snippet
	copyKey(key: string, type: 'public' | 'secret') {
		const text = key;
		const el = document.createElement('textarea');
		el.value = text;
		document.body.appendChild(el);
		el.select();
		document.execCommand('copy');
		type === 'public' ? (this.showPublicCopyText = true) : (this.showSecretCopyText = true);
		setTimeout(() => {
			type === 'public' ? (this.showPublicCopyText = false) : (this.showSecretCopyText = false);
		}, 3000);
		document.body.removeChild(el);
	}
}
