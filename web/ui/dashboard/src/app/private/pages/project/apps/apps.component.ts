import { Component, OnInit } from '@angular/core';
import { FormArray, FormBuilder, FormGroup, Validators } from '@angular/forms';
import { Router } from '@angular/router';
import { APP } from 'src/app/models/app.model';
import { PAGINATION } from 'src/app/models/global.model';

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
	isDeletingApp: boolean = false;
	isCreatingNewApp: boolean = false;
	editAppMode: boolean = false;
	currentAppId!: string;
	apps!: { pagination: PAGINATION; content: APP[] };
	displayedApps: { date: string; content: APP[] }[] = [];
	appsDetailsItem?: any;
	
	constructor(private router: Router, private formBuilder:FormBuilder) {}

	ngOnInit(): void {}

	searchApps(searchDetails: { searchInput?: any; type: 'filter' | 'apps' }) {
		const searchString: string = searchDetails?.searchInput?.target?.value || this.appsSearchString;
	}

	filterAppByStatus(status: string) {
		this.selectedAppStatus = status;
	}

	openUpdateAppModal(app: APP) {
		this.currentAppId = '';
	}

	loadEventsFromAppsTable(appId: string) {}

	viewAppDetails() {
		this.router.navigate(['/projects/1/apps/1'])
	}

	deleteApp(){
		
	}

	
}
