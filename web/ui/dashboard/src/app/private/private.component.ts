import { Component, OnInit } from '@angular/core';
import { Router } from '@angular/router';
import { ORGANIZATION_DATA } from '../models/organisation.model';
import { GeneralService } from '../services/general/general.service';
import { PrivateService } from './private.service';

@Component({
	selector: 'app-private',
	templateUrl: './private.component.html',
	styleUrls: ['./private.component.scss']
})
export class PrivateComponent implements OnInit {
	showDropdown = false;
	showOrgDropdown = false;
	showMoreDropdown = false;
	showOverlay = false;
	apiURL = this.generalService.apiURL();
	organisations!: ORGANIZATION_DATA[];
	userOrganization!: ORGANIZATION_DATA;

	constructor(private generalService: GeneralService, private privateService: PrivateService, private router: Router) {}

	ngOnInit() {
		this.getOrganizations();
	}

	async logout() {
		await this.privateService.logout();
		localStorage.removeItem('CONVOY_AUTH');
		this.router.navigateByUrl('/login');
	}

	authDetails() {
		const authDetails = localStorage.getItem('CONVOY_AUTH');
		return authDetails ? JSON.parse(authDetails) : false;
	}

	requestToken(): string {
		if (this.authDetails()) {
			const { username, password } = this.authDetails();
			return btoa(`${username + ':' + password}`);
		} else {
			return '';
		}
	}

	async getOrganizations() {
		try {
			const response = await this.privateService.getOrganizations();
			this.organisations = response.data;
			const setOrg = localStorage.getItem('ORG_DETAILS');
			if (!setOrg) {
				this.selectOrganisation(this.organisations[0]);
			} else {
				this.userOrganization = JSON.parse(setOrg);
			}
		} catch (error) {}
	}

	selectOrganisation(organisation: ORGANIZATION_DATA) {
		const userOrganisation = organisation;
		this.userOrganization = userOrganisation;
		localStorage.setItem('ORG_DETAILS', JSON.stringify(userOrganisation));
		const organisationId = userOrganisation?.id;
		localStorage.setItem('orgId', organisationId);
		const currentUrl = this.router.url;
		if (currentUrl.includes('/projects/')) {
			this.router.navigateByUrl('/projects');
		} else {
			this.router.navigateByUrl('/', { skipLocationChange: true }).then(() => {
				this.router.navigate([currentUrl]);
			});
		}
		this.showOrgDropdown = false;
	}
}
