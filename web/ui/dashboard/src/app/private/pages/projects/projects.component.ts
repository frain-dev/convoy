import { Component, OnInit } from '@angular/core';
import { GROUP } from 'convoy-dashboard/lib/models/group.model';
import { ProjectsService } from './projects.service';

@Component({
	selector: 'app-projects',
	templateUrl: './projects.component.html',
	styleUrls: ['./projects.component.scss']
})
export class ProjectsComponent implements OnInit {
	projects!: GROUP[];
	constructor(private projectsService: ProjectsService) {}

	ngOnInit(): void {
		this.getProjects();
	}

	async getProjects() {
		try {
			const projectsResponse = await this.projectsService.getProjects();
			this.projects = projectsResponse.data;
		} catch (error) {
			console.log(error);
		}
	}
}
