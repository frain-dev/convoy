import { Component, ElementRef, OnInit, ViewChild } from '@angular/core';
import { CommonModule } from '@angular/common';
import { DialogHeaderComponent, DialogDirective } from 'src/app/components/dialog/dialog.directive';
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
import { NotificationComponent } from 'src/app/components/notification/notification.component';
import { SourceURLComponent } from '../../components/create-source/source-url/source-url.component';

@Component({
	selector: 'convoy-setup-project',
	standalone: true,
	imports: [CommonModule, DialogHeaderComponent, CardComponent, ButtonComponent, CreateSourceModule, CreateSubscriptionModule, CreateEndpointComponent, ToggleComponent, LoaderModule, CardComponent, DialogDirective, NotificationComponent, SourceURLComponent],
	templateUrl: './setup-project.component.html',
	styleUrls: ['./setup-project.component.scss']
})
export class SetupProjectComponent implements OnInit {
	@ViewChild(CreateSourceComponent) createSourceForm!: CreateSourceComponent;
	@ViewChild(CreateEndpointComponent) createEndpointForm!: CreateEndpointComponent;
	@ViewChild(CreateSubscriptionComponent) createSubscriptionForm!: CreateSubscriptionComponent;
	@ViewChild('projectSetupDialog', { static: true }) dialog!: ElementRef<HTMLDialogElement>;
	@ViewChild('sourceURLDialog', { static: true }) sourceURLDialog!: ElementRef<HTMLDialogElement>;

	activeProjectId = this.route.snapshot.params.id;
	projectType: 'incoming' | 'outgoing' = 'outgoing';

	newSource!: SOURCE;
	newEndpoint!: ENDPOINT;
	automaticSubscription = true;
	subscriptionData: any;
	isLoading = false;
	showLoader = false;
	connectPubSub = false;
	sourceURL!: string;

	constructor(public privateService: PrivateService, private generalService: GeneralService, private router: Router, private route: ActivatedRoute, private subscriptionService: CreateSubscriptionService) {}

	async ngOnInit() {
		this.dialog.nativeElement.showModal();
		if (!this.privateService.getProjectDetails?.uid) {
			this.showLoader = true;
			await this.privateService.getProjectDetails;
			this.showLoader = false;
		}

		if (this.privateService.getProjectDetails?.uid) {
			this.projectType = this.privateService.getProjectDetails?.type;
		}
	}

	ngOnDestroy() {
		this.dialog.nativeElement.close();
	}

	cancel() {
		this.dialog.nativeElement.close();
		this.privateService.getProjectDetails?.uid ? this.router.navigateByUrl('/projects/' + this.privateService.getProjectDetails?.uid) : this.router.navigateByUrl('/projects/' + this.activeProjectId);
	}

	onProjectOnboardingComplete() {
		const type = this.privateService.getProjectDetails?.type ?? this.projectType;
		const label = type ? type.charAt(0).toUpperCase() + type.slice(1) : 'Project';
		this.generalService.showNotification({ message: `${label} Configuration Complete`, style: 'success', type: 'modal' });
		this.router.navigateByUrl('/projects/' + this.privateService.getProjectDetails?.uid);
	}

	onCreateSource(newSource: SOURCE) {
		this.createSubscriptionForm.subscriptionForm.patchValue({ source_id: newSource.uid });
		this.sourceURL = newSource.url;
	}

	onCreateEndpoint(newEndpoint: ENDPOINT) {
		this.createSubscriptionForm.subscriptionForm.patchValue({ endpoint_id: newEndpoint.uid });
	}

	toggleFormsLoaders(loaderState: boolean) {
		this.createSubscriptionForm.isCreatingSubscription = loaderState;
		if (this.createSourceForm) this.createSourceForm.isloading = loaderState;
	}

	get canSave(): boolean {
		if (this.isLoading) return false;
		if (!this.createEndpointForm?.addNewEndpointForm?.valid) return false;
		// Incoming: require source form valid (we create the source on Save and Proceed; no id until then)
		if (this.projectType === 'incoming' && this.createSourceForm && !this.createSourceForm.sourceForm?.valid) return false;
		if (this.projectType === 'outgoing' && this.connectPubSub && this.createSourceForm && !this.createSourceForm.sourceForm?.valid) return false;
		if (!this.automaticSubscription && !this.createSubscriptionForm?.subscriptionForm?.valid) return false;
		return true;
	}

	async saveProjectConfig() {
		this.toggleFormsLoaders(true);
		this.createSubscriptionForm.subscriptionForm.patchValue({ name: `${this.createEndpointForm.addNewEndpointForm.value.name}'s Subscription` });
		if (this.projectType === 'outgoing') {
			const fc = this.createSubscriptionForm.subscriptionForm.get('filter_config');
			const et = fc?.get('event_types')?.value;
			if (!et || (Array.isArray(et) && et.length === 0)) {
				this.createSubscriptionForm.selectedEventTypes = ['*'];
				fc?.patchValue({ event_types: ['*'] });
			}
		}
		await this.createSubscriptionForm.runSubscriptionValidation();

		const nameInvalid = this.createSubscriptionForm.subscriptionForm.get('name')?.invalid;
		const retryInvalid = this.createSubscriptionForm.subscriptionForm.get('retry_config')?.invalid;
		const filterInvalid = this.createSubscriptionForm.subscriptionForm.get('filter_config')?.invalid;
		// Don't require source_id here for incoming: we create the source below and patch via onCreateSource
		const sourceInvalid =
			this.projectType === 'incoming' &&
			this.createSourceForm?.sourceCreated &&
			(this.createSubscriptionForm.subscriptionForm.get('source_id')?.invalid || !this.createSubscriptionForm.subscriptionForm.get('source_id')?.value);
		if (nameInvalid || retryInvalid || filterInvalid || sourceInvalid) {
			this.toggleFormsLoaders(false);
			this.createSubscriptionForm.subscriptionForm.markAllAsTouched();
			return;
		}

		const endpointForm = this.createEndpointForm;
		const needSaveEndpoint = endpointForm && !endpointForm.endpointCreated;
		if (needSaveEndpoint) await this.createEndpointForm.saveEndpoint();
		if (this.createSourceForm && !this.createSourceForm.sourceCreated) await this.createSourceForm.saveSource();

		// Incoming requires a source; block if we still don't have one after saveSource
		if (this.projectType === 'incoming' && this.createSourceForm && !this.createSourceForm.sourceCreated) {
			this.toggleFormsLoaders(false);
			this.createSubscriptionForm.subscriptionForm.markAllAsTouched();
			return;
		}
		if (this.projectType === 'outgoing' && this.connectPubSub && !this.createSourceForm.sourceCreated) {
			this.toggleFormsLoaders(false);
			return;
		}

		if (this.createSubscriptionForm.subscriptionForm.invalid) {
			this.createSubscriptionForm.isCreatingSubscription = false;
			this.toggleFormsLoaders(false);
			return this.createSubscriptionForm.subscriptionForm.markAllAsTouched();
		}
		try {
			this.createSubscriptionForm.saveSubscription(true);
		} catch (error) {
			this.toggleFormsLoaders(false);
		}
	}
}
