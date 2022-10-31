import { Component, OnInit } from '@angular/core';
import { Router } from '@angular/router';
import { GROUP } from 'src/app/models/group.model';
import { ORGANIZATION_DATA } from 'src/app/models/organisation.model';
import { PrivateService } from '../../private.service';

@Component({
	selector: 'app-projects',
	templateUrl: './projects.component.html',
	styleUrls: ['./projects.component.scss']
})
export class ProjectsComponent implements OnInit {
	projects: GROUP[] = [];
	isLoadingProjects = false;
	projectsLoaderIndex: number[] = [0, 1, 2, 3, 4];
	organisations: ORGANIZATION_DATA[] = [];
	showOrganisationModal = false;
	isloadingOrganisations = false;

	constructor(private privateService: PrivateService, private router: Router) {}

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
			this.isloadingOrganisations = false;
			if (this.organisations.length > 0) {
				this.privateService.organisationDetails = this.organisations[0];
				return this.getProjects();
			}
			return this.router.navigateByUrl('/get-started');
		} catch (error) {
			this.isloadingOrganisations = true;
			this.isLoadingProjects = false;
		}
	}

	async getProjects() {
		try {
			const projectsResponse = await this.privateService.getProjects();
			this.projects = projectsResponse.data;
			delete this.privateService.activeProjectDetails;
			if (!this.projects.length) this.router.navigateByUrl('/get-started');
			this.isLoadingProjects = false;
		} catch (error) {
			this.isLoadingProjects = false;
		}
	}
}
