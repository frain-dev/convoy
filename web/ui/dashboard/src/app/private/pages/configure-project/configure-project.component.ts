import { Component, OnInit } from '@angular/core';
import { CommonModule } from '@angular/common';
import { PrivateService } from '../../private.service';
import { ModalComponent } from 'src/app/components/modal/modal.component';
import { CardComponent } from 'src/app/components/card/card.component';
import { CreateSourceModule } from '../../components/create-source/create-source.module';
import { CreateSubscriptionModule } from '../../components/create-subscription/create-subscription.module';
import { SdkDocumentationComponent } from '../../components/sdk-documentation/sdk-documentation.component';
import { ButtonComponent } from 'src/app/components/button/button.component';
import { GeneralService } from 'src/app/services/general/general.service';
import { ActivatedRoute, Router } from '@angular/router';
import { CreateEndpointComponent } from '../../components/create-endpoint/create-endpoint.component';

export type STAGES = 'setupSDK' | 'createSource' | 'createEndpoint' | 'createSubscription';

@Component({
	selector: 'convoy-configure-project',
	standalone: true,
	imports: [CommonModule, ModalComponent, CardComponent, ButtonComponent, CreateSourceModule, CreateSubscriptionModule, SdkDocumentationComponent, CreateEndpointComponent],
	templateUrl: './configure-project.component.html',
	styleUrls: ['./configure-project.component.scss']
})
export class ConfigureProjectComponent implements OnInit {
	projectStage: STAGES = 'setupSDK';
	projectStages = [
		{ projectStage: 'Create Endpoint', currentStage: 'pending', id: 'createEndpoint' },
		{ projectStage: 'Create Source', currentStage: 'pending', id: 'createSource' },
		{ projectStage: 'Create Subscription', currentStage: 'pending', id: 'createSubscription' }
	];
	projectType: 'incoming' | 'outgoing' = 'outgoing';
	activeProjectId = this.route.snapshot.params.id;

	constructor(public privateService: PrivateService, private generalService: GeneralService, public router: Router, private route: ActivatedRoute) {}

	ngOnInit() {
		if (this.privateService.activeProjectDetails?.uid) {
			this.projectType = this.privateService.activeProjectDetails?.type;
			this.goToCurrentState(this.privateService.activeProjectDetails?.type);
		}
	}

	async goToCurrentState(projectType: 'incoming' | 'outgoing') {
		if (projectType === 'outgoing') {
			this.projectStages = this.projectStages.filter(e => e.id !== 'createSource');
			this.toggleActiveStage({ project: 'setupSDK' });
		} else {
			this.toggleActiveStage({ project: 'createEndpoint' });
		}
	}

	cancel() {
		this.privateService.activeProjectDetails?.uid ? this.router.navigateByUrl('/projects/' + this.privateService.activeProjectDetails?.uid) : this.router.navigateByUrl('/projects/' + this.activeProjectId);
	}

	onProjectOnboardingComplete() {
		this.generalService.showNotification({ message: `${this.privateService.activeProjectDetails?.type} configuration complete`, style: 'success', type: 'modal' });
		this.router.navigateByUrl('/projects/' + this.privateService.activeProjectDetails?.uid);
	}

	toggleActiveStage(stageDetails: { project: STAGES; prevStage?: STAGES }) {
		this.projectStage = stageDetails.project;
		if (stageDetails.project !== 'setupSDK') {
			this.projectStages.forEach(item => {
				if (item.id === stageDetails.project) item.currentStage = 'current';
				if (item.id === stageDetails.prevStage) item.currentStage = 'done';
			});
		}
		window.scrollTo(0, 0);
	}
}
