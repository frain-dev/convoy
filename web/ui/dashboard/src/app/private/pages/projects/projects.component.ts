import { Component, OnDestroy, OnInit } from '@angular/core';
import { NavigationEnd, Router } from '@angular/router';
import { PROJECT } from 'src/app/models/project.model';
import { PrivateService } from '../../private.service';

@Component({
	selector: 'app-projects',
	templateUrl: './projects.component.html',
	styleUrls: ['./projects.component.scss']
})
export class ProjectsComponent implements OnInit, OnDestroy {
	projects: PROJECT[] = [];
	isLoadingProjects = false;
	projectsLoaderIndex: number[] = [0, 1, 2, 3, 4];
	showOrganisationModal = false;
	reloadSubscription: any;
	isLoadingProject: boolean = false;

	constructor(private privateService: PrivateService, private router: Router) {
		// for reloading this component when the same route is called again
		this.router.routeReuseStrategy.shouldReuseRoute = function () {
			return false;
		};

		this.reloadSubscription = this.router.events.subscribe(event => {
			if (event instanceof NavigationEnd) {
				this.router.navigated = false;
			}
		});
	}

	async ngOnInit() {
		this.getProjects();
	}

	ngOnDestroy(): void {
		this.reloadSubscription?.unsubscribe();
	}

	async getProjects() {
		this.isLoadingProjects = true;

		try {
			const projectsResponse = await this.privateService.getProjects();
			this.projects = projectsResponse.data;
			delete this.privateService.activeProjectDetails;
			this.isLoadingProjects = false;
		} catch (error) {
			this.isLoadingProjects = false;
		}
	}

	// We're calling project details ahead because every page under project has a guard that requires project details to be present and to also prevent multiple calls
	async getProjectCompleteDetails(projectId: string) {
		this.isLoadingProject = true;

		try {
			await this.privateService.getProjectDetails({ refresh: true, projectId }).then(() => this.privateService.getProjectStat({ refresh: true }));
			this.router.navigate([`/projects/${projectId}`]);
		} catch (error) {
			this.isLoadingProject = false;
		}
	}
}
