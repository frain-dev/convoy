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
	showAddAnalytics = false;
	apiURL = this.generalService.apiURL();
	organisations!: ORGANIZATION_DATA[];
	userOrganization!: ORGANIZATION_DATA;

	constructor(private generalService: GeneralService, private router: Router, private privateService: PrivateService) {}

	async ngOnInit() {
		this.getConfiguration();
		await this.getOrganizations();
	}

	async logout() {
		await this.privateService.logout();
		localStorage.removeItem('CONVOY_AUTH');
		localStorage.removeItem('CONVOY_ORG');
		this.router.navigateByUrl('/login');
	}

	authDetails() {
		const authDetails = localStorage.getItem('CONVOY_AUTH');
		return authDetails ? JSON.parse(authDetails) : false;
	}

	async getConfiguration() {
		try {
			const response = await this.privateService.getConfiguration();
			if (response.data.length === 0) this.showAddAnalytics = true;
		} catch {}
	}

	async getOrganizations() {
		try {
			const response = await this.privateService.getOrganizations();
			this.organisations = response.data.content;
			const setOrg = localStorage.getItem('CONVOY_ORG');
			if (!setOrg || setOrg === 'undefined') {
				this.privateService.organisationDetails = this.organisations[0];
				this.userOrganization = this.organisations[0];
				localStorage.setItem('CONVOY_ORG', JSON.stringify(this.organisations[0]));
			} else {
				this.privateService.organisationDetails = JSON.parse(setOrg);
				this.userOrganization = JSON.parse(setOrg);
			}
		} catch (error) {
			return error;
		}
	}

	async selectOrganisation(organisation: ORGANIZATION_DATA) {
		this.privateService.organisationDetails = organisation;
		this.userOrganization = organisation;
		localStorage.setItem('CONVOY_ORG', JSON.stringify(organisation));
		this.showOrgDropdown = false;
		location.reload();
	}

	closeAddOrganisationModal(event?: { action: 'created' | 'cancel' }) {
		this.showAddOrganisationModal = false;
		this.getOrganizations();
		if (event?.action === 'created') this.selectOrganisation(this.userOrganization);
	}
}
