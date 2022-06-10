import { Component, OnInit } from '@angular/core';
import { GROUP } from 'src/app/models/group.model';
import { ProjectsService } from './projects.service';

@Component({
	selector: 'app-projects',
	templateUrl: './projects.component.html',
	styleUrls: ['./projects.component.scss']
})
export class ProjectsComponent implements OnInit {
	projects!: GROUP[];
	isLoadingProjects = false;
	noData = false;
	projectsLoaderIndex: number[] = [0, 1, 2, 3, 4];

	constructor(private projectsService: ProjectsService) {}

	async ngOnInit() {
		await this.getProjects();
	}
	getOrgId() {
		return localStorage.getItem('ORG_ID') || '';
	}
	async getProjects() {
		this.isLoadingProjects = true;
		try {
			const projectsResponse = await this.projectsService.getProjects(this.getOrgId());
			projectsResponse.data.length ? (this.noData = false) : (this.noData = true);
			this.projects = projectsResponse.data;
			this.isLoadingProjects = false;
		} catch (error) {
			this.isLoadingProjects = false;
		}
	}
}
