import { AfterViewInit, Component, ElementRef, OnInit, ViewChild } from '@angular/core';
import { CommonModule } from '@angular/common';
import { FormBuilder, ReactiveFormsModule } from '@angular/forms';
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
import { SelectComponent } from 'src/app/components/select/select.component';

@Component({
    selector: 'convoy-setup-project',
    imports: [CommonModule, ReactiveFormsModule, DialogHeaderComponent, CardComponent, ButtonComponent, CreateSourceModule, CreateSubscriptionModule, CreateEndpointComponent, ToggleComponent, LoaderModule, CardComponent, DialogDirective, NotificationComponent, SourceURLComponent, SelectComponent],
    templateUrl: './setup-project.component.html',
    styleUrls: ['./setup-project.component.scss']
})
export class SetupProjectComponent implements OnInit, AfterViewInit {
	@ViewChild(CreateSourceComponent) createSourceForm!: CreateSourceComponent;
	@ViewChild(CreateEndpointComponent) createEndpointForm!: CreateEndpointComponent;
	@ViewChild(CreateSubscriptionComponent) createSubscriptionForm!: CreateSubscriptionComponent;
	@ViewChild('projectSetupDialog', { static: true }) dialog!: ElementRef<HTMLDialogElement>;
	@ViewChild('sourceURLDialog', { static: true }) sourceURLDialog!: ElementRef<HTMLDialogElement>;

	activeProjectId = this.route.snapshot.params.id;
	projectType: 'incoming' | 'outgoing' = 'outgoing';

	newSource!: SOURCE;
	newEndpoint!: ENDPOINT;
	sources: SOURCE[] = [];
	endpoints: ENDPOINT[] = [];
	endpointOptions: { uid: string; name: string }[] = [];
	selectedSourceId = '';
	selectedEndpointId = '';
	reuseForm = this.formBuilder.group({
		source_id: [''],
		endpoint_id: ['']
	});
	useExistingSource = false;
	useExistingEndpoint = false;
	automaticSubscription = true;
	subscriptionData: any;
	isLoading = false;
	showLoader = false;
	connectPubSub = false;
	sourceURL!: string;

	constructor(public privateService: PrivateService, private generalService: GeneralService, private router: Router, private route: ActivatedRoute, private subscriptionService: CreateSubscriptionService, private formBuilder: FormBuilder) {}

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

		await this.getReusableResources();
	}

	ngAfterViewInit() {
		queueMicrotask(() => this.syncSelectedResourcesToSubscriptionForm());
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
		this.newSource = newSource;
		this.selectedSourceId = newSource.uid;
		this.syncSelectedResourcesToSubscriptionForm();
		this.sourceURL = newSource.url;
	}

	onCreateEndpoint(newEndpoint: ENDPOINT) {
		this.newEndpoint = newEndpoint;
		this.selectedEndpointId = newEndpoint.uid;
		this.syncSelectedResourcesToSubscriptionForm();
	}

	async getReusableResources() {
		try {
			const [endpointsResponse, sourcesResponse] = await Promise.all([this.privateService.getEndpoints(), this.projectType === 'incoming' ? this.privateService.getSources() : Promise.resolve(null)]);

			this.endpoints = endpointsResponse.data.content || [];
			this.endpointOptions = this.endpoints.map(endpoint => ({
				uid: endpoint.uid,
				name: `${endpoint.name} - ${endpoint.target_url || endpoint.url}`
			}));
			if (this.endpoints.length > 0) {
				this.useExistingEndpoint = true;
				this.selectedEndpointId = this.endpoints.length === 1 ? this.endpoints[0].uid : '';
				this.reuseForm.patchValue({ endpoint_id: this.selectedEndpointId });
				this.createSubscriptionForm?.subscriptionForm.patchValue({ endpoint_id: this.selectedEndpointId });
			}

			this.sources = sourcesResponse?.data?.content || [];
			if (this.projectType === 'incoming' && this.sources.length > 0) {
				this.useExistingSource = true;
				this.selectedSourceId = this.sources.length === 1 ? this.sources[0].uid : '';
				this.sourceURL = this.sources.length === 1 ? this.sources[0].url : '';
				this.reuseForm.patchValue({ source_id: this.selectedSourceId });
				this.createSubscriptionForm?.subscriptionForm.patchValue({ source_id: this.selectedSourceId });
			}
		} catch {}
	}

	onSelectExistingSource(sourceId: string) {
		this.selectedSourceId = sourceId;
		this.reuseForm.patchValue({ source_id: sourceId });
		const source = this.sources.find(item => item.uid === sourceId);
		this.sourceURL = source?.url || '';
		this.syncSelectedResourcesToSubscriptionForm();
	}

	onSelectExistingEndpoint(endpointId: string) {
		this.selectedEndpointId = endpointId;
		this.reuseForm.patchValue({ endpoint_id: endpointId });
		this.syncSelectedResourcesToSubscriptionForm();
	}

	syncSelectedResourcesToSubscriptionForm() {
		if (!this.createSubscriptionForm) return;

		this.createSubscriptionForm.subscriptionForm.patchValue({
			endpoint_id: this.useExistingEndpoint ? this.selectedEndpointId : this.createSubscriptionForm.subscriptionForm.get('endpoint_id')?.value,
			source_id: this.useExistingSource ? this.selectedSourceId : this.createSubscriptionForm.subscriptionForm.get('source_id')?.value
		});
	}

	toggleFormsLoaders(loaderState: boolean) {
		this.createSubscriptionForm.isCreatingSubscription = loaderState;
		if (this.createSourceForm) this.createSourceForm.isloading = loaderState;
	}

	get canSave(): boolean {
		this.syncSelectedResourcesToSubscriptionForm();

		return !this.setupSaveHint;
	}

	get setupSaveHint(): string {
		if (this.isLoading) return 'Loading setup details...';
		if (this.useExistingEndpoint && !this.selectedEndpointId) return 'Select an endpoint to continue.';
		if (!this.useExistingEndpoint && !this.createEndpointForm?.addNewEndpointForm?.valid) return 'Create a valid endpoint to continue.';
		// Incoming: require source form valid (we create the source on Save and Proceed; no id until then)
		if (this.projectType === 'incoming' && this.useExistingSource && !this.selectedSourceId) return 'Select a source to continue.';
		if (this.projectType === 'incoming' && !this.useExistingSource && this.createSourceForm && !this.createSourceForm.sourceForm?.valid) return 'Create a valid source to continue.';
		if (this.projectType === 'outgoing' && this.connectPubSub && this.createSourceForm && !this.createSourceForm.sourceForm?.valid) return 'Create a valid source to continue.';
		if (!this.automaticSubscription && this.createSubscriptionForm?.selectedEventTypes.length === 0) return 'Select at least one event type or enable automatic subscription.';
		return '';
	}

	async saveProjectConfig() {
		this.toggleFormsLoaders(true);
		const selectedEndpoint = this.endpoints.find(endpoint => endpoint.uid === this.selectedEndpointId);
		const endpointName = this.useExistingEndpoint ? selectedEndpoint?.name : this.createEndpointForm.addNewEndpointForm.value.name;
		this.createSubscriptionForm.subscriptionForm.patchValue({
			name: `${endpointName}'s Subscription`,
			endpoint_id: this.selectedEndpointId || this.createSubscriptionForm.subscriptionForm.get('endpoint_id')?.value,
			source_id: this.selectedSourceId || this.createSubscriptionForm.subscriptionForm.get('source_id')?.value
		});
		const fc = this.createSubscriptionForm.subscriptionForm.get('filter_config');
		const et = fc?.get('event_types')?.value;
		if (!et || (Array.isArray(et) && et.length === 0)) {
			this.createSubscriptionForm.selectedEventTypes = ['*'];
			fc?.patchValue({ event_types: ['*'] });
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
		const needSaveEndpoint = !this.useExistingEndpoint && endpointForm && !endpointForm.endpointCreated;
		if (needSaveEndpoint) await this.createEndpointForm.saveEndpoint();
		if (!this.useExistingSource && this.createSourceForm && !this.createSourceForm.sourceCreated) await this.createSourceForm.saveSource();
		if (this.createSourceForm?.sourceCreated && this.createSourceForm?.sourceData?.uid) {
			this.createSubscriptionForm.subscriptionForm.patchValue({ source_id: this.createSourceForm.sourceData.uid });
		}

		// Incoming requires a source; block if we still don't have one after saveSource
		if (this.projectType === 'incoming' && !this.useExistingSource && this.createSourceForm && !this.createSourceForm.sourceCreated) {
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
