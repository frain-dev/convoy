import { Component, OnInit, ViewChild } from '@angular/core';
import { Router } from '@angular/router';
import { GROUP } from '../models/group.model';
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
	projects?: GROUP[];
	organisations?: ORGANIZATION_DATA[];
	userOrganization?: ORGANIZATION_DATA;

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
			if (response.data.length === 0 && !this.router.url.includes('app-portal')) this.showAddAnalytics = true;
		} catch {}
	}

	async getOrganizations() {
		try {
			const response = await this.privateService.getOrganizations();
			this.organisations = response.data.content;
			if (this.organisations?.length === 0) return this.router.navigateByUrl('/get-started');
			this.checkForSelectedOrganisation();
			return this.getProjects();
		} catch (error) {
			return error;
		}
	}

	async getProjects() {
		try {
			const projectsResponse = await this.privateService.getProjects();
			this.projects = projectsResponse.data;
			if (this.projects?.length === 0) return this.router.navigateByUrl('/get-started');
			return;
		} catch (error) {
			return error;
		}
	}

	async selectOrganisation(organisation: ORGANIZATION_DATA) {
		this.privateService.organisationDetails = organisation;
		this.userOrganization = organisation;
		localStorage.setItem('CONVOY_ORG', JSON.stringify(organisation));
		this.showOrgDropdown = false;
		location.replace('./projects');
	}

	checkForSelectedOrganisation() {
		if (!this.organisations?.length) return;

		const selectedOrganisation = localStorage.getItem('CONVOY_ORG');
		if (!selectedOrganisation || selectedOrganisation === 'undefined') return this.updateOrganisationDetails();

		const organisationDetails = JSON.parse(selectedOrganisation);
		if (this.organisations.find(org => org.uid === organisationDetails.uid)) {
			this.privateService.organisationDetails = organisationDetails;
			this.userOrganization = organisationDetails;
		} else {
			this.updateOrganisationDetails();
		}
	}

	updateOrganisationDetails() {
		if (!this.organisations?.length) return;

		this.privateService.organisationDetails = this.organisations[0];
		this.userOrganization = this.organisations[0];
		localStorage.setItem('CONVOY_ORG', JSON.stringify(this.organisations[0]));
	}

	closeAddOrganisationModal(event?: { action: 'created' | 'cancel' }) {
		this.showAddOrganisationModal = false;
		this.getOrganizations();
		if (event?.action === 'created' && this.userOrganization) this.selectOrganisation(this.userOrganization);
	}

	get isProjectDetailsPage() {
		return this.router.url.includes('/projects/');
	}

	get showHelpCard() {
		const formUrls = ['apps/new', 'sources/new', 'subscriptions/new'];
		const checkForCreateForms = formUrls.some(url => this.router.url.includes(url));
		return this.router.url === '/projects' || this.router.url === '/projects/new' || checkForCreateForms;
	}
}
