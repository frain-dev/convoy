import { Component, OnInit } from '@angular/core';
import { Router } from '@angular/router';
import { GROUP } from 'src/app/models/group.model';
import { PrivateService } from '../../private.service';

export type STAGES = 'createProject' | 'setupSDK' | 'createSource' | 'createApplication' | 'createSubscription';
@Component({
	selector: 'app-create-project',
	templateUrl: './create-project.component.html',
	styleUrls: ['./create-project.component.scss']
})
export class CreateProjectComponent implements OnInit {
	showInfo = false;
	isLoadingProjects = false;

	constructor(private router: Router, public privateService: PrivateService) {}

	ngOnInit() {
		this.getProjects();
	}

	async createProject(newProjectData: { action: string; data: GROUP }) {
		const projectId = newProjectData.data.uid;
		this.router.navigateByUrl('/projects/' + projectId + '/configure');
	}

	async getProjects() {
		this.isLoadingProjects = true;
		try {
			const projectsResponse = await this.privateService.getProjects();
			const projects = projectsResponse.data;
			this.isLoadingProjects = false;
			if (projects.length === 0) this.showInfo = true;
		} catch (error) {
			this.isLoadingProjects = false;
			return error;
		}
	}

	cancel() {
		this.privateService.activeProjectDetails?.uid ? this.router.navigateByUrl('/projects/' + this.privateService.activeProjectDetails?.uid) : this.router.navigateByUrl('/projects');
	}
}
