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
	showAddOrganisationModal = false;
	apiURL = this.generalService.apiURL();
	organisations!: ORGANIZATION_DATA[];
	userOrganization!: ORGANIZATION_DATA;

	constructor(private generalService: GeneralService, private router: Router, private privateService: PrivateService) {}

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

	async getOrganizations() {
		try {
			const response = await this.privateService.getOrganizations();
			this.organisations = response.data.content;
			const setOrg = localStorage.getItem('CONVOY_ORG');
			if (!setOrg || setOrg === 'undefined') {
				this.selectOrganisation(this.organisations[0]);
			} else {
				this.userOrganization = JSON.parse(setOrg);
			}
		} catch (error) {
			return error;
		}
	}

	async selectOrganisation(organisation: ORGANIZATION_DATA) {
		this.userOrganization = organisation;
		localStorage.setItem('CONVOY_ORG', JSON.stringify(organisation));
		this.showOrgDropdown = false;
		if (this.router.url.includes('/projects/')) {
			this.router.navigateByUrl('/projects');
		} else {
			this.router.navigateByUrl('/', { skipLocationChange: true }).then(() => {
				this.router.navigateByUrl(this.router.url);
			});
		}
	}

	closeAddOrganisationModal() {
		this.showAddOrganisationModal = false;
		this.getOrganizations();
	}
}
