import { Component, OnInit } from '@angular/core';
import { ActivatedRoute, Router } from '@angular/router';
import { GeneralService } from 'src/app/services/general/general.service';
import { SettingsService } from './settings.service';

export type SETTINGS = 'organisation settings' | 'configuration settings' | 'personal access tokens';

@Component({
	selector: 'convoy-settings',
	templateUrl: './settings.component.html',
	styleUrls: ['./settings.component.scss']
})
export class SettingsComponent implements OnInit {
	activePage: SETTINGS = 'organisation settings';
	settingsMenu: { name: SETTINGS; icon: string }[] = [
		{ name: 'organisation settings', icon: 'settings' }
		// hidden for cloud instance
		// { name: 'configuration settings', icon: 'settings' },
	];

	constructor(private router: Router, private route: ActivatedRoute) {}

	ngOnInit() {
		this.toggleActivePage(this.route.snapshot.queryParams?.activePage ?? 'organisation settings');
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
