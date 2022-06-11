import { Location } from '@angular/common';
import { Component, OnInit } from '@angular/core';
import { ActivatedRoute, Router } from '@angular/router';
import { APP } from 'src/app/models/app.model';
import { PAGINATION } from 'src/app/models/global.model';
import { HTTP_RESPONSE } from 'src/app/models/http.model';
import { PrivateService } from 'src/app/private/private.service';
import { GeneralService } from 'src/app/services/general/general.service';

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

	constructor(private router: Router, private route: ActivatedRoute, private generalService: GeneralService, public privateService: PrivateService, private location: Location) {}

	async ngOnInit() {
		await this.getApps();
	}

	goBack() {
		this.location.back();
	}

	searchApps(searchDetails: { searchInput?: any }) {
		const searchString: string = searchDetails?.searchInput?.target?.value || this.appsSearchString;
		this.getApps({ search: searchString });
	}

	filterAppByStatus(status: string) {
		this.selectedAppStatus = status;
	}

	loadEventsFromAppsTable(event: any, appId: string) {
		event.stopPropagation();
		const projectId = this.privateService.activeProjectDetails.uid;
		this.router.navigate(['/projects/' + projectId + '/events'], { queryParams: { eventsApp: appId } });
	}

	deleteApp() {}

	async getApps(requestDetails?: { search?: string; page?: number }): Promise<HTTP_RESPONSE> {
		this.isloadingApps = true;
		const page = requestDetails?.page || this.route.snapshot.queryParams.page || 1;
		try {
			const appsResponse = await this.privateService.getApps({ pageNo: page, searchString: requestDetails?.search });

			this.apps = appsResponse.data;
			this.displayedApps = this.generalService.setContentDisplayed(this.apps.content);
			this.appsDetailsItem = this.apps?.content[0];

			if (!this.filteredApps) this.filteredApps = appsResponse.data.content;

			this.isloadingApps = false;
			return appsResponse;
		} catch (error: any) {
			this.isloadingApps = false;
			return error;
		}
	}
}
