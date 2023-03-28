import { Component, OnInit, ViewChild } from '@angular/core';
import { CommonModule } from '@angular/common';
import { ModalComponent, ModalHeaderComponent } from 'src/app/components/modal/modal.component';
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

@Component({
	selector: 'convoy-setup-project',
	standalone: true,
	imports: [CommonModule, ModalComponent, ModalHeaderComponent, CardComponent, ButtonComponent, CreateSourceModule, CreateSubscriptionModule, CreateEndpointComponent, ToggleComponent, LoaderModule, CardComponent],
	templateUrl: './setup-project.component.html',
	styleUrls: ['./setup-project.component.scss']
})
export class SetupProjectComponent implements OnInit {
	@ViewChild(CreateSourceComponent) createSourceForm!: CreateSourceComponent;
	@ViewChild(CreateEndpointComponent) createEndpointForm!: CreateEndpointComponent;
	@ViewChild(CreateSubscriptionComponent) createSubscriptionForm!: CreateSubscriptionComponent;
	activeProjectId = this.route.snapshot.params.id;
	projectType: 'incoming' | 'outgoing' = 'outgoing';

	newSource!: SOURCE;
	newEndpoint!: ENDPOINT;
	automaticSubscription = true;
	subscriptionData: any;
	isLoading = false;
	showLoader = false;
	connectPubSub = false;

	constructor(public privateService: PrivateService, private generalService: GeneralService, private router: Router, private route: ActivatedRoute, private subscriptionService: CreateSubscriptionService) {}

	async ngOnInit() {
		if (!this.privateService.activeProjectDetails?.uid) {
			this.showLoader = true;
			await this.privateService.getProjectDetails();
			this.showLoader = false;
		}

		if (this.privateService.activeProjectDetails?.uid) {
			this.projectType = this.privateService.activeProjectDetails?.type;
		}
	}

	cancel() {
		this.privateService.activeProjectDetails?.uid ? this.router.navigateByUrl('/projects/' + this.privateService.activeProjectDetails?.uid) : this.router.navigateByUrl('/projects/' + this.activeProjectId);
	}

	onProjectOnboardingComplete() {
		this.generalService.showNotification({ message: `${this.privateService.activeProjectDetails?.type} configuration complete`, style: 'success', type: 'modal' });

		this.router.navigateByUrl('/projects/' + this.privateService.activeProjectDetails?.uid);
	}

	async saveProjectConfig() {
		const [sourceDetails, endpointDetails] = await Promise.allSettled([this.createSourceForm && !this.createSourceForm?.sourceCreated ? this.createSourceForm.saveSource() : false, !this.createEndpointForm.endpointCreated ? this.createEndpointForm.saveEndpoint() : false]);

		if (this.projectType === 'incoming' && sourceDetails.status === 'fulfilled' && typeof sourceDetails.value !== 'boolean') {
			this.newSource = sourceDetails.value?.data;
			this.subscriptionService.subscriptionData = { source_id: sourceDetails.value?.data.uid };
		}

		if (endpointDetails.status === 'fulfilled' && typeof endpointDetails.value !== 'boolean') {
			this.newEndpoint = endpointDetails.value?.data;
			this.subscriptionService.subscriptionData = { ...this.subscriptionService.subscriptionData, endpoint_id: endpointDetails.value?.data.uid };
		}

		if (this.automaticSubscription) this.subscriptionService.subscriptionData = { ...this.subscriptionService.subscriptionData, name: `${this.newEndpoint.title}${this.newSource ? ' → ' + this.newSource.name : ''}'s Subscription` };
		await this.createSubscriptionForm.saveSubscription(true);
	}
}
