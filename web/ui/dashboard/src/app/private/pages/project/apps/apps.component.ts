import { Location } from '@angular/common';
import { Component, OnInit } from '@angular/core';
import { ActivatedRoute, Router } from '@angular/router';
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
	showCreateAppModal = this.router.url.split('/')[4] === 'new';
	showEditAppModal = this.router.url.split('/')[4] === 'edit';
	isloadingApps: boolean = false;
	isDeletingApp: boolean = false;
	isCreatingNewApp: boolean = false;
	editAppMode: boolean = false;
	currentAppId!: string;
	apps!: { pagination: PAGINATION; content: APP[] };
	displayedApps: { date: string; content: APP[] }[] = [];
	appsDetailsItem?: any;
	appsPage: number = 1;
	filteredApps!: APP[];
	constructor(private router: Router, private route: ActivatedRoute, private generalService: GeneralService, private appService: AppsService, private location:Location) {}

	async ngOnInit() {
		await this.getApps();
	}

	goBack(){
		this.location.back()
	}

	searchApps(searchDetails: { searchInput?: any }) {
		const searchString: string = searchDetails?.searchInput?.target?.value || this.appsSearchString;
		this.getApps({ search: searchString });
	}

	filterAppByStatus(status: string) {
		this.selectedAppStatus = status;
	}

	openUpdateAppModal(app: APP) {
		this.currentAppId = '';
	}

	loadEventsFromAppsTable(appId: string) {
		const projectId = this.appService.projectId;
		this.router.navigate(['/projects/' + projectId + '/events'], { queryParams: { eventsApp: appId } });
	}

	deleteApp() {}

	async getApps(requestDetails?: { search?: string; page?: number }): Promise<HTTP_RESPONSE> {
		this.isloadingApps = true;
		const page = requestDetails?.page || this.route.snapshot.queryParams.page || 1;
		try {
			const appsResponse = await this.appService.getApps({ pageNo: page, searchString: requestDetails?.search });

			this.apps = appsResponse.data;
			this.displayedApps = this.generalService.setContentDisplayed(this.apps.content);
			this.appsDetailsItem = this.apps?.content[0];

			if (!this.filteredApps) this.filteredApps = appsResponse.data.content;

			// if (this.updateAppDetail) this.appsDetailsItem = this.apps.content.find(item => this.appsDetailsItem?.uid == item.uid);

			this.isloadingApps = false;
			return appsResponse;
		} catch (error: any) {
			this.isloadingApps = false;
			return error;
		}
	}
}
