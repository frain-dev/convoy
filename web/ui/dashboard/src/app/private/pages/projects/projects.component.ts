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
	isLoadingProjects: boolean = false;
	projectsLoaderIndex: number[] = [0, 1, 2, 3, 4];
	constructor(private projectsService: ProjectsService) {}

	ngOnInit(): void {
		this.getProjects();
	}

	async getProjects() {
		this.isLoadingProjects = true;
		try {
			const projectsResponse = await this.projectsService.getProjects();
			this.projects = projectsResponse.data;
			this.isLoadingProjects = false;
		} catch (error) {
			console.log(error);
			this.isLoadingProjects = false;
		}
	}
}
