import { Component, OnInit } from '@angular/core';
import { Router } from '@angular/router';
import { APP } from 'src/app/models/app.model';
import { PAGINATION } from 'src/app/models/global.model';
import { HTTP_RESPONSE } from 'src/app/models/http.model';
import { GeneralService } from 'src/app/services/general/general.service';
import { AppsService } from './apps.service';

@Component({
	selector: 'app-apps',
	templateUrl: './apps.component.html',
	styleUrls: ['./apps.component.scss']
})
export class AppsComponent implements OnInit {
	appsTableHead: string[] = ['Status', 'Name', 'Time Created', 'Updated', 'Events', 'Endpoints'];
	appsSearchString: string = '';
	selectedAppStatus: string = 'All';
	appStatuses: string[] = ['All', 'Enabled', 'Disabled'];
	showOverlay: boolean = false;
	showAppStatusDropdown: boolean = false;
	showAppDetails: boolean = false;
	showDeleteAppModal: boolean = false;
	showCreateAppModal: boolean = false;
	isloadingApps: boolean = false;
	isloadingMoreApps: boolean = false;
	isDeletingApp: boolean = false;
	isCreatingNewApp: boolean = false;
	editAppMode: boolean = false;
	currentAppId!: string;
	apps!: { pagination: PAGINATION; content: APP[] };
	displayedApps: { date: string; content: APP[] }[] = [];
	appsDetailsItem?: any;
	appsPage: number = 1;
	filteredApps!: APP[];
	constructor(private router: Router, private generalService:GeneralService, private appService:AppsService) {}

	async ngOnInit() {
		await this.getApps({type: 'apps'})
	}

	searchApps(searchDetails: { searchInput?: any; type: 'filter' | 'apps' }) {
		const searchString: string = searchDetails?.searchInput?.target?.value || this.appsSearchString;
	}

	filterAppByStatus(status: string) {
		this.selectedAppStatus = status;
	}

	openUpdateAppModal(app: APP) {
		this.currentAppId = '';
	}

	async loadEventsFromAppsTable(appId: string) {
		this.router.navigate(['/'])
		// await this.getEvents({ addToURL: true, appId: appId, fromFilter: true });
		// this.toggleActiveTab('events');
	}

	viewAppDetails() {
		this.router.navigate(['/projects/1/apps/1'])
	}

	deleteApp(){
		
	}

	async getApps(requestDetails?: { search?: string; type: 'filter' | 'apps' }): Promise<HTTP_RESPONSE> {
		if (this.apps?.pagination?.next === this.appsPage) this.isloadingMoreApps = true;
		if (requestDetails?.type === 'apps') this.isloadingApps = true;

		try {
			const appsResponse = await this.appService.getApps({ pageNo: this.appsPage || 1, searchString: requestDetails?.search });

			if (!requestDetails?.search && this.apps?.pagination?.next === this.appsPage) {
				const content = [...this.apps.content, ...appsResponse.data.content];
				const pagination = appsResponse.data.pagination;
				this.apps = { content, pagination };
				this.displayedApps = this.generalService.setContentDisplayed(this.apps.content);
				this.isloadingMoreApps = false;
				return appsResponse;
			}

			if (requestDetails?.type === 'apps') {
				this.apps = appsResponse.data;
				this.displayedApps = this.generalService.setContentDisplayed(this.apps.content);
				this.appsDetailsItem = this.apps?.content[0];
				console.log(this.displayedApps)
			}

			if (!this.filteredApps) this.filteredApps = appsResponse.data.content;

			// if (this.updateAppDetail) this.appsDetailsItem = this.apps.content.find(item => this.appsDetailsItem?.uid == item.uid);

			this.isloadingApps = false;
			return appsResponse;
		} catch (error: any) {
			this.isloadingApps = false;
			this.isloadingMoreApps = false;
			return error;
		}
	}

	
}
