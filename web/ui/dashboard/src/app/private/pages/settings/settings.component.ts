import { Location } from '@angular/common';
import { Component, OnInit } from '@angular/core';
import { ActivatedRoute, Router } from '@angular/router';
import { LicensesService } from 'src/app/services/licenses/licenses.service';
import { HttpService } from 'src/app/services/http/http.service';

export type SETTINGS = 'organisation settings' | 'configuration settings' | 'personal access tokens' | 'team' | 'usage and billing';

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
		{ name: 'team', icon: 'team', svg: 'stroke' }
	];

	constructor(
		private router: Router, 
		private route: ActivatedRoute, 
		public licenseService: LicensesService,
		private httpService: HttpService
	) {}

	ngOnInit() {
		this.checkBillingStatus();
		if (this.licenseService.hasLicense('CREATE_ORG_MEMBER')) this.toggleActivePage(this.route.snapshot.queryParams?.activePage ?? 'organisation settings');
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
		// Start with base menu items (without billing)
		this.settingsMenu = [
			{ name: 'organisation settings', icon: 'org', svg: 'fill' },
			{ name: 'team', icon: 'team', svg: 'stroke' }
		];

		// Only add billing menu item if enabled
		if (this.billingEnabled) {
			this.settingsMenu.push({ name: 'usage and billing', icon: 'status', svg: 'stroke' });
		}

		// If current active page is billing but billing is disabled, switch to organisation settings
		if (this.activePage === 'usage and billing' && !this.billingEnabled) {
			this.toggleActivePage('organisation settings');
		}
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
