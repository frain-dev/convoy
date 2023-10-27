import { Component, OnInit } from '@angular/core';
import { PROJECT } from 'src/app/models/project.model';
import { PrivateService } from '../../private.service';

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

	constructor(private privateService: PrivateService) {
		this.privateService.projects$.subscribe(projects => (this.projects = projects.data));
	}

	async ngOnInit() {
		this.isLoadingProjects = true;
		await this.getProjects();
		this.isLoadingProjects = false;
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
