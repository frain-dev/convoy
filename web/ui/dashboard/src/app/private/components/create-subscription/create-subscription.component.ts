import { Component, EventEmitter, Input, OnInit, Output, ViewChild, inject } from '@angular/core';
import { FormBuilder, FormGroup, Validators } from '@angular/forms';
import { ActivatedRoute, Router } from '@angular/router';
import { APP, ENDPOINT } from 'src/app/models/endpoint.model';
import { SOURCE } from 'src/app/models/source.model';
import { PrivateService } from '../../private.service';
import { CreateEndpointComponent } from '../create-endpoint/create-endpoint.component';
import { CreateSourceComponent } from '../create-source/create-source.component';
import { CreateSubscriptionService } from './create-subscription.service';
import { RbacService } from 'src/app/services/rbac/rbac.service';

@Component({
	selector: 'convoy-create-subscription',
	templateUrl: './create-subscription.component.html',
	styleUrls: ['./create-subscription.component.scss'],
	providers: []
})
export class CreateSubscriptionComponent implements OnInit {
	@Output() onAction = new EventEmitter();
	@Input('action') action: 'update' | 'create' | 'view' = 'create';
	@Input('isPortal') isPortal: 'true' | 'false' = 'false';
	@Input('subscriptionId') subscriptionId = this.route.snapshot.params.id || this.route.snapshot.queryParams.id;
	@Input('showAction') showAction: 'true' | 'false' = 'false';

	@ViewChild(CreateEndpointComponent) createEndpointForm!: CreateEndpointComponent;
	@ViewChild(CreateSourceComponent) createSourceForm!: CreateSourceComponent;

	subscriptionForm: FormGroup = this.formBuilder.group({
		name: [null, Validators.required],
		source_id: [''],
		endpoint_id: [null, Validators.required],
		function: [null],
		retry_config: this.formBuilder.group({
			type: [],
			retry_count: [null, Validators.pattern('^[-+]?[0-9]+$')],
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
	configSetting = this.route.snapshot.queryParams.configSetting;
	retryLogicTypes = [
		{ uid: 'linear', name: 'Linear time retry' },
		{ uid: 'exponential', name: 'Exponential time backoff' }
	];
	isCreatingSubscription = false;

	projectType?: 'incoming' | 'outgoing' = this.privateService.getProjectDetails?.type;
	isLoadingForm = true;
	isLoadingPortalProject = false;
	token: string = this.route.snapshot.queryParams.token;
	showError = false;

	configurations = [
		{ uid: 'filter_config', name: 'Filter', show: false },
		{ uid: 'tranform_config', name: 'Transform', show: false },
		{ uid: 'retry_config', name: 'Retry Logic', show: false }
	];
	createdSubscription = false;
	private rbacService = inject(RbacService);
	showFilterDialog = false;
	showTransformDialog = false;

	constructor(private formBuilder: FormBuilder, private privateService: PrivateService, private createSubscriptionService: CreateSubscriptionService, private route: ActivatedRoute, private router: Router) {}

	async ngOnInit() {
		this.isLoadingForm = true;
		if (this.isPortal !== 'true' && this.showAction === 'true') await Promise.all([this.getEndpoints(), this.getSources()]);
		if (this.action === 'update') await this.getSubscriptionDetails();
		this.isLoadingForm = false;

		// add required validation on source input for incoming projects
		if (this.projectType === 'incoming') {
			this.subscriptionForm.get('source_id')?.addValidators(Validators.required);
			this.subscriptionForm.get('source_id')?.updateValueAndValidity();
		} else {
			this.configurations.push({ uid: 'events', name: 'Event Types', show: false });
		}

		if (this.configSetting) this.toggleConfigForm(this.configSetting, true);
		if (!(await this.rbacService.userCanAccess('Subscriptions|MANAGE'))) this.subscriptionForm.disable();
	}

	toggleConfig(configValue: string) {
		this.action === 'view' ? this.router.navigate(['/projects/' + this.privateService.getProjectDetails?.uid + '/subscriptions/' + this.subscriptionId], { queryParams: { configSetting: configValue } }) : this.toggleConfigForm(configValue);
	}

	toggleConfigForm(configValue: string, value?: boolean) {
		this.configurations.forEach(config => {
			if (config.uid === configValue) config.show = value ? value : !config.show;
		});
	}

	showConfig(configValue: string): boolean {
		return this.configurations.find(config => config.uid === configValue)?.show || false;
	}

	async getSubscriptionDetails() {
		try {
			const response = await this.createSubscriptionService.getSubscriptionDetail(this.subscriptionId);
			this.subscriptionForm.patchValue(response.data);
			this.subscriptionForm.patchValue({ source_id: response.data?.source_metadata?.uid, endpoint_id: response.data?.endpoint_metadata?.uid });
			if (response.data.filter_config?.event_types) {
				this.eventTags = response.data.filter_config?.event_types;
				if (this.eventTags.length > 1 || this.eventTags[0] !== '*') this.toggleConfigForm('events');
			} else this.eventTags = [];
			const filterConfig = response.data.filter_config?.filter;

			if (this.action === 'update' && (Object.keys(filterConfig.body).length > 0 || Object.keys(filterConfig.headers).length > 0)) {
				this.configurations.forEach(config => {
					if (config.uid === 'filter_config') config.show = true;
				});
			}

			if (this.token) this.projectType = 'outgoing';

			if (response.data?.retry_config) this.toggleConfigForm('retry_config');
			if (response.data?.function) this.toggleConfigForm('tranform_config');

			return;
		} catch (error) {
			return error;
		}
	}

	async getEndpoints() {
		try {
			const response = await this.privateService.getEndpoints();
			this.endpoints = this.token ? response.data : response.data.content;
			this.modifyEndpointData(this.token ? response.data : response.data.content);
			return;
		} catch (error) {
			return error;
		}
	}

	async getSources() {
		if (this.privateService.getProjectDetails?.type === 'outgoing' || this.token) return;

		try {
			const sourcesResponse = await this.privateService.getSources();
			this.sources = sourcesResponse.data.content;
			return;
		} catch (error) {
			return;
		}
	}

	async onCreateSource(newSource: SOURCE) {
		this.subscriptionForm.patchValue({ source_id: newSource.uid });
		await this.getSources();
	}

	async onCreateEndpoint(newEndpoint: ENDPOINT) {
		this.subscriptionForm.patchValue({ endpoint_id: newEndpoint.uid });
		await this.getEndpoints();
	}

	toggleFormsLoaders(loaderState: boolean) {
		this.isCreatingSubscription = loaderState;
		if (this.createEndpointForm) this.createEndpointForm.savingEndpoint = loaderState;
		if (this.createSourceForm) this.createSourceForm.isloading = loaderState;
	}

	async runSubscriptionValidation() {
		const configFields: any = {
			retry_config: ['retry_config.type', 'retry_config.retry_count', 'retry_config.duration'],
			events: ['filter_config.event_types']
		};
		this.configurations.forEach(config => {
			const fields = configFields[config.uid];
			if (this.showConfig(config.uid)) {
				fields?.forEach((item: string) => {
					this.subscriptionForm.get(item)?.addValidators(Validators.required);
					this.subscriptionForm.get(item)?.updateValueAndValidity();
				});
			} else {
				fields?.forEach((item: string) => {
					this.subscriptionForm.get(item)?.removeValidators(Validators.required);
					this.subscriptionForm.get(item)?.updateValueAndValidity();
				});
			}
		});
		return;
	}

	async saveSubscription(setup?: boolean) {
		this.toggleFormsLoaders(true);
		if (this.eventTags.length === 0) this.subscriptionForm.patchValue({ filter_config: { event_types: ['*'] } });

		await this.runSubscriptionValidation();

		if (this.subscriptionForm.get('name')?.invalid || this.subscriptionForm.get('retry_config')?.invalid || this.subscriptionForm.get('filter_config')?.invalid) {
			this.toggleFormsLoaders(false);
			this.subscriptionForm.markAllAsTouched();
			return;
		}

		if (this.createEndpointForm && !this.createEndpointForm.endpointCreated) await this.createEndpointForm.saveEndpoint();
		if (this.createSourceForm && !this.createSourceForm.sourceCreated) await this.createSourceForm.saveSource();

		// check subscription form validation
		if (this.subscriptionForm.invalid) {
			this.isCreatingSubscription = false;
			return this.subscriptionForm.markAllAsTouched();
		}

		// check if configs are added, else delete the properties
		const subscriptionData = structuredClone(this.subscriptionForm.value);
		const retryDuration = this.subscriptionForm.get('retry_config.duration');
		this.configurations[2]?.show ? (subscriptionData.retry_config.duration = retryDuration?.value + 's') : delete subscriptionData.retry_config;

		// create subscription
		try {
			const response = this.action == 'update' ? await this.createSubscriptionService.updateSubscription({ data: subscriptionData, id: this.subscriptionId }) : await this.createSubscriptionService.createSubscription(subscriptionData);
			if (setup) await this.privateService.getProjectStat({ refresh: true });
			this.privateService.getSubscriptions();
			localStorage.removeItem('FUNCTION');
			this.onAction.emit({ data: response.data, action: this.action == 'update' ? 'update' : 'create' });
			this.createdSubscription = true;
		} catch (error) {
			this.createdSubscription = false;
			this.isCreatingSubscription = false;
		}
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

	goToSubsriptionsPage() {
		this.router.navigateByUrl('/projects/' + this.privateService.getProjectDetails?.uid + '/subscriptions');
	}

	setupFilter() {
		document.getElementById(this.showAction === 'true' ? 'subscriptionForm' : 'configureProjectForm')?.scroll({ top: 0, behavior: 'smooth' });
		this.showFilterDialog = true;
	}

	setupTransformDialog() {
		document.getElementById(this.showAction === 'true' ? 'subscriptionForm' : 'configureProjectForm')?.scroll({ top: 0, behavior: 'smooth' });
		this.showTransformDialog = true;
	}

	getFilterSchema(schema: any) {
		if (schema.headerSchema) this.subscriptionForm.get('filter_config.filter.headers')?.patchValue(schema.headerSchema);
		if (schema.bodySchema) this.subscriptionForm.get('filter_config.filter.body')?.patchValue(schema.bodySchema);
		this.showFilterDialog = false;
	}

	getFunction(subscriptionFunction: any) {
		if (subscriptionFunction) this.subscriptionForm.get('function')?.patchValue(subscriptionFunction);
		this.showTransformDialog = false;
	}

	get shouldShowBorder(): number {
		return this.configurations.filter(config => config.show).length;
	}
}
