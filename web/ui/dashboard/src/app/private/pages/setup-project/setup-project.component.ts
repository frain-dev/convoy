import { Component, ElementRef, OnInit, ViewChild } from '@angular/core';
import { CommonModule } from '@angular/common';
import { ModalHeaderComponent, DialogDirective } from 'src/app/components/modal/modal.component';
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
	imports: [CommonModule, ModalHeaderComponent, CardComponent, ButtonComponent, CreateSourceModule, CreateSubscriptionModule, CreateEndpointComponent, ToggleComponent, LoaderModule, CardComponent, DialogDirective],
	templateUrl: './setup-project.component.html',
	styleUrls: ['./setup-project.component.scss']
})
export class SetupProjectComponent implements OnInit {
	@ViewChild(CreateSourceComponent) createSourceForm!: CreateSourceComponent;
	@ViewChild(CreateEndpointComponent) createEndpointForm!: CreateEndpointComponent;
	@ViewChild(CreateSubscriptionComponent) createSubscriptionForm!: CreateSubscriptionComponent;
    @ViewChild('projectSetupDialog', { static: true }) dialog!: ElementRef<HTMLDialogElement>;

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
        this.dialog.nativeElement.showModal()
		if (!this.privateService.getProjectDetails?.uid) {
			this.showLoader = true;
			await this.privateService.getProjectDetails;
			this.showLoader = false;
		}

		if (this.privateService.getProjectDetails?.uid) {
			this.projectType = this.privateService.getProjectDetails?.type;
		}
	}

    ngOnDestroy(){
        this.dialog.nativeElement.close()
    }

	cancel() {
        this.dialog.nativeElement.close()
		this.privateService.getProjectDetails?.uid ? this.router.navigateByUrl('/projects/' + this.privateService.getProjectDetails?.uid) : this.router.navigateByUrl('/projects/' + this.activeProjectId);
	}

	onProjectOnboardingComplete() {
		this.generalService.showNotification({ message: `${this.privateService.getProjectDetails?.type} configuration complete`, style: 'success', type: 'modal' });

		this.router.navigateByUrl('/projects/' + this.privateService.getProjectDetails?.uid);
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
		this.createSubscriptionForm.subscriptionForm.patchValue({ name: `${this.createEndpointForm.addNewEndpointForm.value.name}${this.projectType === 'incoming' ? ' → ' + this.createSourceForm.sourceForm.value.name : ''}'s Subscription` });
		await this.createSubscriptionForm.runSubscriptionValidation();

		if (this.createSubscriptionForm.subscriptionForm.get('name')?.invalid || this.createSubscriptionForm.subscriptionForm.get('retry_config')?.invalid || this.createSubscriptionForm.subscriptionForm.get('filter_config')?.invalid) {
			this.toggleFormsLoaders(false);
			this.createSubscriptionForm.subscriptionForm.markAllAsTouched();
			return;
		}

		if (this.createEndpointForm && !this.createEndpointForm.endpointCreated) await this.createEndpointForm.saveEndpoint();
		if (this.createSourceForm && !this.createSourceForm.sourceCreated) await this.createSourceForm.saveSource();

		if (this.projectType === 'outgoing' && this.connectPubSub && !this.createSourceForm.sourceCreated) return;

		// check subscription form validation
		if (this.createSubscriptionForm.subscriptionForm.invalid) {
			this.createSubscriptionForm.isCreatingSubscription = false;
			return this.createSubscriptionForm.subscriptionForm.markAllAsTouched();
		}

		// create subscription
		try {
			this.createSubscriptionForm.saveSubscription(true);
		} catch (error) {}
	}
}
