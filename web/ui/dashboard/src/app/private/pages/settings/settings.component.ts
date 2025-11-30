import {Component, OnInit} from '@angular/core';
import {ActivatedRoute, Router} from '@angular/router';
import {LicensesService} from 'src/app/services/licenses/licenses.service';
import {HttpService} from 'src/app/services/http/http.service';

export type SETTINGS = 'organisation settings' | 'configuration settings' | 'personal access tokens' | 'team' | 'usage and billing' | 'early adopter features';

@Component({
	selector: 'convoy-settings',
	templateUrl: './settings.component.html',
	styleUrls: ['./settings.component.scss']
})
export class SettingsComponent implements OnInit {
	activePage: SETTINGS = 'organisation settings';
	billingEnabled = false;
	settingsMenu: { name: SETTINGS; icon: string; svg: 'stroke' | 'fill' }[] = [
		{ name: 'organisation settings', icon: 'org', svg: 'fill' },
		{ name: 'team', icon: 'team', svg: 'stroke' },
		{ name: 'early adopter features', icon: 'settings', svg: 'fill' }
		// { name: 'configuration settings', icon: 'settings', svg: 'fill' }
	];

	constructor(
		private router: Router,
		private route: ActivatedRoute,
		public licenseService: LicensesService,
		private httpService: HttpService
	) {}

	ngOnInit() {
		this.checkBillingStatus();
		// Set active page from URL query parameter with license validation
		const requestedPage = this.route.snapshot.queryParams?.activePage ?? 'organisation settings';
		this.setActivePageWithLicenseCheck(requestedPage);
	}

	private async checkBillingStatus() {
		try {
			const response = await this.httpService.request({
				url: '/billing/enabled',
				method: 'get',
				hideNotification: true
			});
			this.billingEnabled = response.data?.enabled || false;
			this.updateSettingsMenu();
		} catch (error) {
			console.warn('Failed to check billing status:', error);
			this.billingEnabled = false;
			this.updateSettingsMenu();
		}
	}

	private updateSettingsMenu() {
		this.settingsMenu = [
			{ name: 'organisation settings', icon: 'org', svg: 'fill' },
			{ name: 'team', icon: 'team', svg: 'stroke' },
			{ name: 'early adopter features', icon: 'settings', svg: 'fill' }
		];

		if (this.billingEnabled) {
			this.settingsMenu.push({ name: 'usage and billing', icon: 'status', svg: 'stroke' });
		}

		if (this.activePage === 'usage and billing' && !this.billingEnabled) {
			this.toggleActivePage('organisation settings');
		}
	}

	setActivePageWithLicenseCheck(requestedPage: string) {
		// Validate license requirements for specific pages
		if (requestedPage === 'team' && !this.licenseService.hasLicense('CREATE_USER')) {
			// Redirect to organisation settings if user doesn't have team management license
			this.toggleActivePage('organisation settings');
			return;
		}

		// For other pages, allow navigation (they have their own license checks)
		this.toggleActivePage(requestedPage as SETTINGS);
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
