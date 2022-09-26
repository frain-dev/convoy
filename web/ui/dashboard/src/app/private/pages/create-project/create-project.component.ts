import { Location } from '@angular/common';
import { Component, OnInit } from '@angular/core';
import { Router } from '@angular/router';
import { GROUP } from 'src/app/models/group.model';
import { GeneralService } from 'src/app/services/general/general.service';
import { PrivateService } from '../../private.service';

export type STAGES = 'createProject' | 'setupSDK' | 'createSource' | 'createApplication' | 'createSubscription';
@Component({
	selector: 'app-create-project',
	templateUrl: './create-project.component.html',
	styleUrls: ['./create-project.component.scss']
})
export class CreateProjectComponent implements OnInit {
	projectStage: STAGES = 'createProject';
	projectStages = [
		{ projectStage: 'Create Application', currentStage: 'pending', id: 'createApplication' },
		{ projectStage: 'Create Source', currentStage: 'pending', id: 'createSource' },
		{ projectStage: 'Create Subscription', currentStage: 'pending', id: 'createSubscription' }
	];
	hashAlgorithms = ['SHA256', 'SHA512', 'MD5', 'SHA1', 'SHA224', 'SHA384', 'SHA3_224', 'SHA3_256', 'SHA3_384', 'SHA3_512', 'SHA512_256', 'SHA512_224'];
	retryLogicTypes = [
		{ id: 'linear', type: 'Linear time retry' },
		{ id: 'exponential', type: 'Exponential time backoff' }
	];
	projectType: 'incoming' | 'outgoing' = 'outgoing';
	projects!: GROUP[];
	tabs = [
		{ label: 'Javascript', id: 'javascript' },
		{ label: 'Python', id: 'python' },
		{ label: 'PHP', id: 'php' },
		{ label: 'Ruby', id: 'ruby' },
		{ label: 'Golang', id: 'golang' }
	];
	activeTab = 'javascript';
	showInfo = false;

	constructor(private router: Router, private location: Location, public privateService: PrivateService, private generalService: GeneralService) {}

	ngOnInit() {
		this.getProjects();

		if (this.privateService.activeProjectDetails?.uid) {
			this.toggleActiveStage({ project: 'setupSDK' });
			this.projectType = this.privateService.activeProjectDetails?.type;
		}
	}

	async createProject(newProjectData: { action: string; data: GROUP }) {
		this.projectType = newProjectData.data.type;
		newProjectData.data.type === 'incoming' ? (this.projectType = 'incoming') : (this.projectType = 'outgoing');
		if (newProjectData.data.type === 'outgoing') this.projectStages = this.projectStages.filter(e => e.id !== 'createSource');
		this.toggleActiveStage({ project: 'setupSDK' });
	}

	async getProjects() {
		try {
			const projectsResponse = await this.privateService.getProjects();
			this.projects = projectsResponse.data;
			if (this.projects.length === 0) this.showInfo = true;
		} catch (error) {
			return error;
		}
	}

	cancel() {
		this.location.back();
	}

	onProjectOnboardingComplete() {
		this.generalService.showNotification({ message: 'Project setup complete', style: 'success' });
		this.router.navigateByUrl('/projects/' + this.privateService.activeProjectDetails.uid);
	}

	toggleActiveStage(stageDetails: { project: STAGES; prevStage?: STAGES }) {
		this.projectStage = stageDetails.project;
		if (stageDetails.project !== 'setupSDK') {
			this.projectStages.forEach(item => {
				if (item.id === stageDetails.project) item.currentStage = 'current';
				if (item.id === stageDetails.prevStage) item.currentStage = 'done';
			});
		}
	}

	switchTabs(activeTab: string) {
		switch (activeTab) {
			case 'javascript':
				this.activeTab = 'javascript';
				// this.fetchPageData('convoy-js');
				break;
			case 'python':
				this.activeTab = 'python';
				// this.fetchPageData('convoy-pyhton');
				break;
			case 'php':
				this.activeTab = 'php';
				// this.fetchPageData('convoy-php');
				break;
			case 'ruby':
				this.activeTab = 'ruby';
				// this.fetchPageData('convoy-ruby');
				break;
			case 'golang':
				this.activeTab = 'golang';
				// this.fetchPageData('convoy-ruby');
				break;
			default:
				break;
		}
	}
}
