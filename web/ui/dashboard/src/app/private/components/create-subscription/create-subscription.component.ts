import { Component, EventEmitter, Input, OnInit, Output, ViewChild } from '@angular/core';
import { FormBuilder, FormGroup, Validators } from '@angular/forms';
import { ActivatedRoute, Router } from '@angular/router';
import { APP, ENDPOINT } from 'src/app/models/endpoint.model';
import { SOURCE } from 'src/app/models/group.model';
import { FormatSecondsPipe } from 'src/app/pipes/formatSeconds/format-seconds.pipe';
import { PrivateService } from '../../private.service';
import { CreateEndpointComponent } from '../create-endpoint/create-endpoint.component';
import { CreateSourceComponent } from '../create-source/create-source.component';
import { CreateSubscriptionService } from './create-subscription.service';

@Component({
	selector: 'convoy-create-subscription',
	templateUrl: './create-subscription.component.html',
	styleUrls: ['./create-subscription.component.scss'],
	providers: [FormatSecondsPipe]
})
export class CreateSubscriptionComponent implements OnInit {
	@Output() onAction = new EventEmitter();
	@Input('action') action: 'update' | 'create' | 'view' = 'create';
	@Input('showAction') showAction: 'true' | 'false' = 'false';

	@ViewChild(CreateEndpointComponent) createEndpointForm!: CreateEndpointComponent;
	@ViewChild(CreateSourceComponent) createSourceForm!: CreateSourceComponent;
	subscriptionForm: FormGroup = this.formBuilder.group({
		name: [null, Validators.required],
		source_id: [''],
		endpoint_id: [null, Validators.required],
		retry_config: this.formBuilder.group({
			type: [],
			retry_count: [],
			duration: []
		}),
		filter_config: this.formBuilder.group({
			event_types: [null],
			filter: this.formBuilder.group({
				headers: [null],
				body: [null]
			})
		})
	});
	endpoints!: ENDPOINT[];
	eventTags: string[] = [];
	apps!: APP[];
	sources!: SOURCE[];
	endPoints: ENDPOINT[] = [];
	showCreateSourceForm = false;
	showCreateEndpointForm = false;
	enableMoreConfig = false;
	showFilterForm = false;
	retryLogicTypes = [
		{ uid: 'linear', name: 'Linear time retry' },
		{ uid: 'exponential', name: 'Exponential time backoff' }
	];
	isCreatingSubscription = false;

	projectType!: 'incoming' | 'outgoing';
	isLoadingForm = true;
	subscriptionId = this.route.snapshot.params.id || this.route.snapshot.queryParams.id;
	isLoadingPortalProject = false;
	token: string = this.route.snapshot.queryParams.token;
	showError = false;
	confirmModal = false;

	configurations = [
		{ uid: 'filter_config', name: 'Filter', show: false },
		{ uid: 'retry_config', name: 'Retry Logic', show: false },
		{ uid: 'events', name: 'Event Types', show: false }
	];
	createdSubscription = false;

	constructor(private formBuilder: FormBuilder, private privateService: PrivateService, private createSubscriptionService: CreateSubscriptionService, private route: ActivatedRoute, private router: Router, private formatSeconds: FormatSecondsPipe) {}

	async ngOnInit() {
		this.isLoadingForm = true;
		await Promise.all([, this.getGetProjectDetails(), this.getSubscriptionDetails()]);
		this.isLoadingForm = false;

		// add required validation on source input for incoming projects
		if (this.projectType === 'incoming') {
			this.subscriptionForm.get('source_id')?.addValidators(Validators.required);
			this.subscriptionForm.get('source_id')?.updateValueAndValidity();
			this.configurations.splice(2, 1);
		}
	}

	toggleConfigForm(configValue: string) {
		this.configurations.forEach(config => {
			if (config.uid === configValue) config.show = !config.show;
		});

		this.onToggleConfig();
	}

	showConfig(configValue: string): boolean {
		return this.configurations.find(config => config.uid === configValue)?.show || false;
	}

	async getSubscriptionDetails() {
		await Promise.all([this.getEndpoints(), this.getSources()]);
		if (this.action === 'create') return;

		try {
			const response = await this.createSubscriptionService.getSubscriptionDetail(this.subscriptionId, this.token);
			this.subscriptionForm.patchValue(response.data);
			this.subscriptionForm.patchValue({ source_id: response.data?.source_metadata?.uid, endpoint_id: response.data?.endpoint_metadata?.uid });
			response.data.filter_config?.event_types ? (this.eventTags = response.data.filter_config?.event_types) : (this.eventTags = []);
			if (this.token) this.projectType = 'outgoing';
			if (response.data?.retry_config) {
				const duration = this.formatSeconds.transform(response.data.retry_config.duration);
				this.subscriptionForm.patchValue({
					retry_config: {
						duration: duration
					}
				});
			}
			return;
		} catch (error) {
			return error;
		}
	}

	async getEndpoints() {
		try {
			const response = await this.createSubscriptionService.getEndpoints({ token: this.token });
			this.endpoints = this.token ? response.data : response.data.content;
			this.modifyEndpointData(this.token ? response.data : response.data.content);
		} catch (error) {
			return error;
		}
	}

	async getSources() {
		if (this.privateService.activeProjectDetails?.type === 'outgoing' || this.token) return;

		try {
			const sourcesResponse = await this.privateService.getSources();
			this.sources = sourcesResponse.data.content;
			return;
		} catch (error) {
			return;
		}
	}

	async getGetProjectDetails() {
		if (this.token) return;

		try {
			const response = await this.privateService.getProjectDetails();
			this.projectType = response.data.type;
			return;
		} catch (error) {
			return;
		}
	}

	async onCreateSource(newSource: SOURCE) {
		await this.getSources();
		this.subscriptionForm.patchValue({ source_id: newSource.uid });
	}

	async onCreateEndpoint(newEndpoint: ENDPOINT) {
		await this.getEndpoints();
		this.subscriptionForm.patchValue({ endpoint_id: newEndpoint.uid });
	}

	onToggleConfig() {
		const retryControls = Object.keys((this.subscriptionForm.get('retry_config') as FormGroup).controls);

		if (this.showConfig('retry_config')) {
			retryControls.forEach(key => this.subscriptionForm.get(`retry_config.${key}`)?.setValidators(Validators.required));
			retryControls.forEach(key => this.subscriptionForm.get(`retry_config.${key}`)?.updateValueAndValidity());
		} else {
			retryControls.forEach(key => this.subscriptionForm.get(`retry_config.${key}`)?.removeValidators(Validators.required));
			retryControls.forEach(key => this.subscriptionForm.get(`retry_config.${key}`)?.updateValueAndValidity());
		}
	}

	async saveSubscription() {
		this.isCreatingSubscription = true;

		// if subscription service has subscription data, use it to update subscription form, else, create endpoint and source
		if (this.createSubscriptionService.subscriptionData) {
			this.subscriptionForm.patchValue(this.createSubscriptionService.subscriptionData);
		} else {
			// trigger create endpoint and source together
			const [endpointDetails, sourceDetails] = await Promise.allSettled([
				this.showCreateEndpointForm && !this.createEndpointForm.endpointCreated ? this.createEndpointForm.saveEndpoint() : false,
				this.showCreateSourceForm && !this.createSourceForm.sourceCreated ? this.createSourceForm.saveSource() : false
			]);
			if (endpointDetails.status === 'fulfilled' && typeof endpointDetails.value !== 'boolean') this.subscriptionForm.patchValue({ endpoint_id: endpointDetails.value?.data.uid });
			if (sourceDetails.status === 'fulfilled' && typeof sourceDetails.value !== 'boolean') this.subscriptionForm.patchValue({ source_id: sourceDetails.value?.data.uid });
		}

		// set filter config
		this.subscriptionForm.patchValue({
			filter_config: { event_types: this.eventTags.length > 0 ? this.eventTags : ['*'] }
		});

		// check subscription form validation
		if (this.subscriptionForm.invalid) {
			this.isCreatingSubscription = false;
			return this.subscriptionForm.markAllAsTouched();
		}

		// check if configs are added, else delete the properties
		const subscriptionData = structuredClone(this.subscriptionForm.value);
		const retryDuration = this.subscriptionForm.get('retry_config.duration');
		this.configurations[1].show ? (subscriptionData.retry_config.duration = retryDuration?.value + 's') : delete subscriptionData.retry_config;

		// create subscription
		try {
			const response =
				this.action == 'update' ? await this.createSubscriptionService.updateSubscription({ data: this.subscriptionForm.value, id: this.subscriptionId, token: this.token }) : await this.createSubscriptionService.createSubscription(subscriptionData, this.token);
			this.onAction.emit({ data: response.data, action: this.action == 'update' ? 'update' : 'create' });
			this.createdSubscription = true;
		} catch (error) {
			this.createdSubscription = false;
			this.isCreatingSubscription = false;
		}
	}

	removeEventTag(tag: string) {
		this.eventTags = this.eventTags.filter(e => e !== tag);
	}

	addTag() {
		const addTagInput = document.getElementById('tagInput');
		const addTagInputValue = document.getElementById('tagInput') as HTMLInputElement;
		addTagInput?.addEventListener('keydown', e => {
			const key = e.keyCode || e.charCode;
			if (key == 8) {
				e.stopImmediatePropagation();
				if (this.eventTags.length > 0 && !addTagInputValue?.value) this.eventTags.splice(-1);
			}
			if (e.which === 188 || e.key == ' ') {
				if (this.eventTags.includes(addTagInputValue?.value)) {
					addTagInputValue.value = '';
					this.eventTags = this.eventTags.filter(e => String(e).trim());
				} else {
					this.eventTags.push(addTagInputValue?.value);
					addTagInputValue.value = '';
					this.eventTags = this.eventTags.filter(e => String(e).trim());
				}
				e.preventDefault();
			}
		});
	}

	focusInput() {
		document.getElementById('tagInput')?.focus();
	}

	modifyEndpointData(endpoints?: ENDPOINT[]) {
		if (endpoints) {
			const endpointData = endpoints;
			endpointData.forEach(data => {
				data.name = data.title;
			});
			this.endPoints = endpointData;
		}
	}

	cancel() {
		document.getElementById(this.router.url.includes('/configure') ? 'configureProjectForm' : 'subscriptionForm')?.scroll({ top: 0, behavior: 'smooth' });
		this.confirmModal = true;
	}

	goToSubsriptionsPage() {
		this.router.navigateByUrl('/projects/' + this.privateService.activeProjectDetails?.uid + '/subscriptions');
	}

	setupFilter() {
		if (this.action === 'view') this.router.navigateByUrl('/projects/' + this.privateService.activeProjectDetails?.uid + '/subscriptions/' + this.subscriptionId);
		document.getElementById(this.showAction === 'true' ? 'subscriptionForm' : 'configureProjectForm')?.scroll({ top: 0, behavior: 'smooth' });
		this.showFilterForm = true;
	}

	getFilterSchema(schema: any) {
		if (schema.headerSchema) this.subscriptionForm.get('filter_config.filter.headers')?.patchValue(schema.headerSchema);
		if (schema.bodySchema) this.subscriptionForm.get('filter_config.filter.body')?.patchValue(schema.bodySchema);

		this.showFilterForm = false;
	}
}
