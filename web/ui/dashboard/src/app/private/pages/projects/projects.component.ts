import { Component, OnDestroy, OnInit } from '@angular/core';
import { NavigationEnd, Router } from '@angular/router';
import { GROUP } from 'src/app/models/group.model';
import { PrivateService } from '../../private.service';

@Component({
	selector: 'app-projects',
	templateUrl: './projects.component.html',
	styleUrls: ['./projects.component.scss']
})
export class ProjectsComponent implements OnInit, OnDestroy {
	projects: GROUP[] = [];
	isLoadingProjects = false;
	projectsLoaderIndex: number[] = [0, 1, 2, 3, 4];
	showOrganisationModal = false;
	reloadSubscription: any;

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
}
