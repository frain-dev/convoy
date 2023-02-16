import { Component, OnInit, ViewChild } from '@angular/core';
import { CommonModule } from '@angular/common';
import { ModalComponent } from 'src/app/components/modal/modal.component';
import { CardComponent } from 'src/app/components/card/card.component';
import { ButtonComponent } from 'src/app/components/button/button.component';
import { CreateSourceModule } from '../../components/create-source/create-source.module';
import { CreateSubscriptionModule } from '../../components/create-subscription/create-subscription.module';
import { CreateEndpointComponent } from '../../components/create-endpoint/create-endpoint.component';
import { PrivateService } from '../../private.service';
import { ActivatedRoute, Router } from '@angular/router';
import { GeneralService } from 'src/app/services/general/general.service';
import { ToggleComponent } from 'src/app/components/toggle/toggle.component';
import { SOURCE } from 'src/app/models/group.model';
import { ENDPOINT } from 'src/app/models/endpoint.model';
import { CreateSourceComponent } from '../../components/create-source/create-source.component';
import { CreateSubscriptionComponent } from '../../components/create-subscription/create-subscription.component';
import { CreateSubscriptionService } from '../../components/create-subscription/create-subscription.service';
import { LoaderModule } from '../../components/loader/loader.module';

export type STAGES = 'createSource' | 'createEndpoint' | 'createSubscription';

@Component({
	selector: 'convoy-setup-project',
	standalone: true,
	imports: [CommonModule, ModalComponent, CardComponent, ButtonComponent, CreateSourceModule, CreateSubscriptionModule, CreateEndpointComponent, ToggleComponent, LoaderModule],
	templateUrl: './setup-project.component.html',
	styleUrls: ['./setup-project.component.scss']
})
export class SetupProjectComponent implements OnInit {
	@ViewChild(CreateSourceComponent) createSourceForm!: CreateSourceComponent;
	@ViewChild(CreateEndpointComponent) createEndpointForm!: CreateEndpointComponent;
	@ViewChild(CreateSubscriptionComponent) createSubscriptionForm!: CreateSubscriptionComponent;
	projectStage: STAGES = 'createSource';
	activeProjectId = this.route.snapshot.params.id;
	projectType: 'incoming' | 'outgoing' = 'outgoing';
	projectStages = [
		{ projectStage: 'Create Source', currentStage: 'pending', id: 'createSource' },
		{ projectStage: 'Create Endpoint', currentStage: 'pending', id: 'createEndpoint' },
		{ projectStage: 'Subscribe Endpoint', currentStage: 'pending', id: 'createSubscription' }
	];
	newSource!: SOURCE;
	newEndpoint!: ENDPOINT;
	automaticSubscription = true;
	subscriptionData: any;
	isLoading = false;
	showLoader = false;

	constructor(public privateService: PrivateService, private generalService: GeneralService, private router: Router, private route: ActivatedRoute, private subscriptionService: CreateSubscriptionService) {}

	async ngOnInit() {
		if (!this.privateService.activeProjectDetails?.uid) {
			this.showLoader = true;
			await this.privateService.getProjectDetails();
			this.showLoader = false;
		}

		if (this.privateService.activeProjectDetails?.uid) {
			this.projectType = this.privateService.activeProjectDetails?.type;
			if (this.projectType === 'outgoing') this.projectStage = 'createEndpoint';
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
		this.projectStages.forEach(item => {
			if (item.id === stageDetails.project) item.currentStage = 'current';
			if (item.id === stageDetails.prevStage) item.currentStage = 'done';
		});
		switch (stageDetails.project) {
			case 'createSource':
				this.createSourceForm.saveSource();
				break;
			case 'createEndpoint':
				this.createEndpointForm.saveEndpoint();
				break;
			case 'createSubscription':
				if (this.automaticSubscription) this.subscriptionData = { ...this.subscriptionData, name: `${this.newEndpoint.title}${this.newSource ? ' - ' + this.newSource.name : ''}`, type: 'noop' };
				this.subscriptionService.subscriptionData = this.subscriptionData;
				this.createSubscriptionForm.saveSubscription();
				break;
			default:
				break;
		}
	}

	onCreateSource(newSource: SOURCE) {
		this.newSource = newSource;
		this.subscriptionData = { source_id: this.newSource.uid };
		this.subscriptionService.subscriptionData = this.subscriptionData;
		this.toggleActiveStage({ project: 'createEndpoint', prevStage: 'createSource' });
	}

	onCreateEndpoint(newEndpoint: ENDPOINT) {
		this.newEndpoint = newEndpoint;
		this.subscriptionData = { ...this.subscriptionData, endpoint_id: this.newEndpoint.uid };
		this.subscriptionService.subscriptionData = this.subscriptionData;
		this.toggleActiveStage({ project: 'createSubscription', prevStage: 'createEndpoint' });
	}

	toggleAutomaticSubscription(event: any) {
		this.automaticSubscription = !this.automaticSubscription;
	}

	saveProjectConfig() {
		this.projectType === 'incoming' ? this.createSourceForm.saveSource() : this.createEndpointForm.saveEndpoint();
	}
}
