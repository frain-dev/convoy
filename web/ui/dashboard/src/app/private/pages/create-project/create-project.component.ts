import { Component, OnInit } from '@angular/core';
import { Router } from '@angular/router';
import { GROUP } from 'src/app/models/group.model';
import { GeneralService } from 'src/app/services/general/general.service';
import { PrivateService } from '../../private.service';

@Component({
	selector: 'app-create-project',
	templateUrl: './create-project.component.html',
	styleUrls: ['./create-project.component.scss']
})
export class CreateProjectComponent implements OnInit {
	projectStage: 'createProject' | 'createSource' | 'createApplication' | 'createSubscription' = 'createProject';
	hashAlgorithms = ['SHA256', 'SHA512', 'MD5', 'SHA1', 'SHA224', 'SHA384', 'SHA3_224', 'SHA3_256', 'SHA3_384', 'SHA3_512', 'SHA512_256', 'SHA512_224'];
	retryLogicTypes = [
		{ id: 'linear', type: 'Linear time retry' },
		{ id: 'exponential', type: 'Exponential time backoff' }
	];
	projectType: 'incoming' | 'outgoing' = 'outgoing';

	constructor(private router: Router, public privateService: PrivateService, private generalService: GeneralService) {}

	ngOnInit(): void {}

	async createProject(newProjectData: GROUP) {
		this.projectType = newProjectData.type;
		newProjectData.type === 'incoming' ? (this.projectStage = 'createSource') : (this.projectStage = 'createApplication');
	}

	cancel() {
		this.router.navigate(['/projects']);
	}

	onProjectOnboardingComplete() {
		this.generalService.showNotification({ message: 'Project setup complete', style: 'success' });
		this.router.navigateByUrl('/projects/' + this.privateService.activeProjectDetails.uid);
	}
}
