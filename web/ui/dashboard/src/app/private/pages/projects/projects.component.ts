import { Component, OnInit } from '@angular/core';
import { PROJECT } from 'src/app/models/project.model';
import { PrivateService } from '../../private.service';
import { Router } from '@angular/router';
import { LicensesService } from 'src/app/services/licenses/licenses.service';

@Component({
	selector: 'app-projects',
	templateUrl: './projects.component.html',
	styleUrls: ['./projects.component.scss']
})
export class ProjectsComponent implements OnInit {
	projects: PROJECT[] = [];
	isLoadingProjects = false;
	projectsLoaderIndex: number[] = [0, 1, 2, 3, 4];
	showOrganisationModal = false;
	isLoadingProject: boolean = false;

	constructor(private privateService: PrivateService, private router: Router, public licenseService: LicensesService) {
		this.privateService.projects$.subscribe(projects => (this.projects = projects.data));
	}

	async ngOnInit() {
		await this.getProjects();
	}

	async getProject(projectId: string) {
		this.isLoadingProjects = true;

		try {
			await this.privateService.getProject({ refresh: true, projectId });
			await this.privateService.getProjectStat({ refresh: true });

			this.router.navigate([`/projects/${projectId}`]);
			this.isLoadingProjects = false;
		} catch (error) {
			this.isLoadingProjects = false;
		}
	}

	async getProjects(): Promise<any> {
		this.isLoadingProjects = true;

		try {
			const response = await this.privateService.getProjects();
			this.projects = response.data;
			this.isLoadingProjects = false;
		} catch (error) {
			this.isLoadingProjects = false;
			return error;
		}
	}
}
