import { Location } from '@angular/common';
import { Component, OnInit } from '@angular/core';
import { ActivatedRoute, Router } from '@angular/router';
import { LicensesService } from 'src/app/services/licenses/licenses.service';

export type SETTINGS = 'organisation settings' | 'configuration settings' | 'personal access tokens' | 'team';

@Component({
	selector: 'convoy-settings',
	templateUrl: './settings.component.html',
	styleUrls: ['./settings.component.scss']
})
export class SettingsComponent implements OnInit {
	activePage: SETTINGS = 'organisation settings';
	settingsMenu: { name: SETTINGS; icon: string; svg: 'stroke' | 'fill' }[] = [
		{ name: 'organisation settings', icon: 'org', svg: 'fill' },
		{ name: 'team', icon: 'team', svg: 'stroke' }
		// { name: 'configuration settings', icon: 'settings', svg: 'fill' }
	];

	constructor(private router: Router, private route: ActivatedRoute, public licenseService: LicensesService) {}

	ngOnInit() {
		if (this.licenseService.hasLicense('CREATE_ORG_MEMBER')) this.toggleActivePage(this.route.snapshot.queryParams?.activePage ?? 'organisation settings');
	}

	toggleActivePage(activePage: SETTINGS) {
		this.activePage = activePage;
		if (!this.router.url.split('/')[2]) this.addPageToUrl();
	}

	addPageToUrl() {
		const queryParams: any = {};
		queryParams.activePage = this.activePage;
		this.router.navigate([], { queryParams: Object.assign({}, queryParams) });
	}
}
