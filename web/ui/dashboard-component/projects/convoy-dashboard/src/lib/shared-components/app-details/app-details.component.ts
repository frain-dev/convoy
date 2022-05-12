import { ChangeDetectionStrategy, Component, Input, Output, EventEmitter, OnInit } from '@angular/core';

@Component({
	selector: 'app-details',
	changeDetection: ChangeDetectionStrategy.OnPush,
	templateUrl: './app-details.component.html',
	styleUrls: ['../../convoy-dashboard.component.scss']
})
export class AppDetailsComponent implements OnInit {
	constructor() {}
	@Input('appsDetailsItem') appsDetailsItem!: any;
	@Input('appPortalLink') appPortalLink!: string;
	@Output() loadEventsFromAppTable = new EventEmitter<string>();
	@Output() openAddEndpointModal = new EventEmitter<any>();
	@Output() closeOverviewRender = new EventEmitter<any>();
	@Output() openEndpointSecretKey = new EventEmitter<string>();
	@Output() editAppDetails = new EventEmitter<any>();
	@Output() fetchAppPortalToken = new EventEmitter<any>();
	@Output() openAddEventModal = new EventEmitter<any>();

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
