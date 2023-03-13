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
	constructor(private router: Router, public privateService: PrivateService) {}

	ngOnInit() {}

	async createProject(newProjectData: { action: string; data: GROUP }) {
		const projectId = newProjectData.data.uid;
		this.router.navigateByUrl('/projects/' + projectId + '/setup');
	}

	cancel() {
		this.router.navigate(['../']);
	}
}
