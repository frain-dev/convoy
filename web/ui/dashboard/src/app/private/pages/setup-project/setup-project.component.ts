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
import { SOURCE } from 'src/app/models/source.model';
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

	onCreateSource(newSource: SOURCE) {
		this.createSubscriptionForm.subscriptionForm.patchValue({ source_id: newSource.uid });
	}

	onCreateEndpoint(newEndpoint: ENDPOINT) {
		this.createSubscriptionForm.subscriptionForm.patchValue({ endpoint_id: newEndpoint.uid });
	}

	toggleFormsLoaders(loaderState: boolean) {
		this.createSubscriptionForm.isCreatingSubscription = loaderState;
		if (this.createEndpointForm) this.createEndpointForm.savingEndpoint = loaderState;
		if (this.createSourceForm) this.createSourceForm.isloading = loaderState;
	}

	async saveProjectConfig() {
		this.toggleFormsLoaders(true);
		if (this.createSubscriptionForm.subscriptionForm.get('name')?.invalid) this.createSubscriptionForm.subscriptionForm.patchValue({ name: 'New Subscription' });
		await this.createSubscriptionForm.runSubscriptionValidation();

		if (this.createSubscriptionForm.subscriptionForm.get('name')?.invalid || this.createSubscriptionForm.subscriptionForm.get('retry_config')?.invalid || this.createSubscriptionForm.subscriptionForm.get('filter_config')?.invalid) {
			this.toggleFormsLoaders(false);
			this.createSubscriptionForm.subscriptionForm.markAllAsTouched();
			return;
		}

		if (this.createEndpointForm && !this.createEndpointForm.endpointCreated) await this.createEndpointForm.saveEndpoint();
		if (this.createSourceForm && !this.createSourceForm.sourceCreated) await this.createSourceForm.saveSource();

		// check subscription form validation
		if (this.createSubscriptionForm.subscriptionForm.invalid) {
			this.createSubscriptionForm.isCreatingSubscription = false;
			return this.createSubscriptionForm.subscriptionForm.markAllAsTouched();
		}

		// create subscription
		try {
			this.createSubscriptionForm.saveSubscription();
		} catch (error) {}
	}
}
