import { Location } from '@angular/common';
import { Component, HostListener, OnInit } from '@angular/core';
import { ActivatedRoute, Router } from '@angular/router';
import { APP, ENDPOINT } from 'src/app/models/app.model';
import { PAGINATION } from 'src/app/models/global.model';
import { PrivateService } from 'src/app/private/private.service';
import { GeneralService } from 'src/app/services/general/general.service';
import { AppDetailsService } from './app-details.service';

@Component({
	selector: 'app-app-details',
	templateUrl: './app-details.component.html',
	styleUrls: ['./app-details.component.scss']
})
export class AppDetailsComponent implements OnInit {
	showAddEndpointModal = false;
	showAddEventModal = false;
	showEndpointSecret = false;
	showPublicCopyText = false;
	showSecretCopyText = false;
	isSendingNewEvent = false;
	savingEndpoint = false;
	loadingAppPotalToken = false;
	isLoadingAppDetails = false;
	shouldRenderSmallSize = false;
	showDeleteModal = false;
	editMode = false;
	isDeletingEndpoint = false;
	screenWidth = window.innerWidth;
	appPortalLink!: string;
	endpointSecretKey!: string;
	appsDetailsItem!: APP;
	apps!: { pagination: PAGINATION; content: APP[] };
	selectedEndpoint?: ENDPOINT;

	constructor(
		private appDetailsService: AppDetailsService,
		private generalService: GeneralService,
		private route: ActivatedRoute,
		private location: Location,
		private router: Router,
		public privateService: PrivateService
	) {}

	ngOnInit() {
		this.getAppDetails(this.route.snapshot.params.id);
		this.checkScreenSize();
	}

	goBack() {
		this.location.back();
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

	viewEndpointSecretKey(secretKey: string) {
		this.showEndpointSecret = !this.showEndpointSecret;
		this.endpointSecretKey = secretKey;
	}

	async getAppDetails(appId: string) {
		this.selectedEndpoint = undefined;
		this.editMode = false;
		this.isLoadingAppDetails = true;

		try {
			const response = await this.appDetailsService.getApp(appId);
			this.appsDetailsItem = response.data;
			this.getAppPortalToken({ redirect: false });
			this.isLoadingAppDetails = false;
		} catch {
			this.isLoadingAppDetails = false;
		}
	}

	async getAppPortalToken(requestDetail: { redirect: boolean }) {
		this.loadingAppPotalToken = true;

		try {
			const appTokenResponse = await this.appDetailsService.getAppPortalToken({ appId: this.appsDetailsItem.uid });
			this.appPortalLink = `<iframe style="width: 100%; height: 100vh; border: none;" src="${appTokenResponse.data.url}"></iframe>`;
			if (requestDetail.redirect) window.open(`${appTokenResponse.data.url}`, '_blank');
			this.loadingAppPotalToken = false;
		} catch (error) {
			this.loadingAppPotalToken = false;
			return error;
		}
	}

	loadEventsFromAppsTable(appId: string) {
		const projectId = this.appDetailsService.projectId;
		this.router.navigate(['/projects/' + projectId + '/events'], { queryParams: { eventsApp: appId } });
	}

	async deleteEndpoint() {
		this.isDeletingEndpoint = true;
		try {
			const response = await this.appDetailsService.deleteEndpoint({ appId: this.appsDetailsItem?.uid, endpointId: this.selectedEndpoint?.uid || '' });
			this.generalService.showNotification({ style: 'success', message: response.message });
			this.showDeleteModal = false;
			this.isDeletingEndpoint = false;
			this.getAppDetails(this.appsDetailsItem?.uid);
		} catch {
			this.isDeletingEndpoint = false;
		}
	}

	checkScreenSize() {
		this.screenWidth > 1010 ? (this.shouldRenderSmallSize = false) : (this.shouldRenderSmallSize = true);
	}

	focusInput() {
		document.getElementById('tagInput')?.focus();
	}

	@HostListener('window:resize', ['$event'])
	onWindowResize() {
		this.screenWidth = window.innerWidth;
		this.checkScreenSize();
	}
}
