import { Component, OnInit } from '@angular/core';
import { Router } from '@angular/router';
import { GROUP } from 'src/app/models/group.model';
import { ORGANIZATION_DATA } from 'src/app/models/organisation.model';
import { PrivateService } from '../../private.service';
import { ProjectsService } from './projects.service';

@Component({
	selector: 'app-projects',
	templateUrl: './projects.component.html',
	styleUrls: ['./projects.component.scss']
})
export class ProjectsComponent implements OnInit {
	projects: GROUP[] = [];
	isLoadingProjects = false;
	noData = false;
	projectsLoaderIndex: number[] = [0, 1, 2, 3, 4];
	organisations: ORGANIZATION_DATA[] = [];
	showOrganisationModal = false;
	isloadingOrganisations = false;

	constructor(private projectsService: ProjectsService, private privateService: PrivateService) {}

	async ngOnInit() {
		this.isloadingOrganisations = true;
		this.getOrganisations();
	}

	async getOrganisations() {
		this.isloadingOrganisations = true;
		this.isLoadingProjects = true;

		try {
			const organisations = await this.privateService.getOrganizations();
			this.organisations = organisations.data.content;
			this.privateService.organisationDetails = this.organisations[0];
			this.isloadingOrganisations = false;
			if (this.organisations.length > 0) return this.getProjects();
		} catch (error) {
			this.isloadingOrganisations = true;
			this.isLoadingProjects = false;
		}
	}

	async getProjects() {
		try {
			const projectsResponse = await this.projectsService.getProjects();
			projectsResponse.data.length > 0 ? (this.noData = false) : (this.noData = true);
			this.projects = projectsResponse.data;
			this.isLoadingProjects = false;
		} catch (error) {
			this.isLoadingProjects = false;
		}
	}

	async setOrganisation() {
		localStorage.setItem('CONVOY_ORG', JSON.stringify(this.organisations[0]));
		this.showOrganisationModal = false;

		// temporary fix for reloading page
		location.reload();
	}
}
