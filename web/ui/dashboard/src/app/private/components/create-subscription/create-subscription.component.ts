import { Component, ElementRef, EventEmitter, Input, OnInit, Output, ViewChild, ViewChildren, QueryList, AfterViewInit, ChangeDetectorRef, inject } from '@angular/core';
import { FormBuilder, FormGroup, FormArray, Validators } from '@angular/forms';
import { ActivatedRoute, Router } from '@angular/router';
import { APP, ENDPOINT } from 'src/app/models/endpoint.model';
import { SOURCE } from 'src/app/models/source.model';
import { PrivateService } from '../../private.service';
import { CreateEndpointComponent } from '../create-endpoint/create-endpoint.component';
import { CreateSourceComponent } from '../create-source/create-source.component';
import { CreateSubscriptionService } from './create-subscription.service';
import { RbacService } from 'src/app/services/rbac/rbac.service';
import { SUBSCRIPTION } from 'src/app/models/subscription';
import { LicensesService } from 'src/app/services/licenses/licenses.service';
import { EVENT_TYPE } from 'src/app/models/event.model';
import { FILTER, FILTER_CREATE_REQUEST } from 'src/app/models/filter.model';
import { FilterService } from './filter.service';

@Component({
	selector: 'convoy-create-subscription',
	templateUrl: './create-subscription.component.html',
	styleUrls: ['./create-subscription.component.scss'],
	providers: []
})
export class CreateSubscriptionComponent implements OnInit, AfterViewInit {
	@Output() onAction = new EventEmitter();
	@Input('action') action: 'update' | 'create' | 'view' = 'create';
	@Input('isPortal') isPortal: 'true' | 'false' = 'false';
	@Input('subscriptionId') subscriptionId = this.route.snapshot.params.id || this.route.snapshot.queryParams.id;
	@Input('endpointId') endpointId: string = this.route.snapshot.queryParams.endpointId;
	@Input('showAction') showAction: 'true' | 'false' = 'false';

	@ViewChild(CreateEndpointComponent) createEndpointForm!: CreateEndpointComponent;
	@ViewChild(CreateSourceComponent) createSourceForm!: CreateSourceComponent;
	@ViewChild('sourceURLDialog', { static: true }) sourceURLDialog!: ElementRef<HTMLDialogElement>;
	@ViewChild('filterDialog', { static: true }) filterDialog!: ElementRef<HTMLDialogElement>;

	@ViewChildren('eventTypeSelect') eventTypeSelects!: QueryList<any>;

	subscriptionForm: FormGroup = this.formBuilder.group({
		name: ['', Validators.required],
		source_id: [''],
		endpoint_id: [null, Validators.required],
		function: [null],
		filter_config: this.formBuilder.group({
			event_types: [null],
			filter: this.formBuilder.group({
				headers: [null],
				body: [null]
			})
		}),
		eventTypes: this.formBuilder.group({})
	});
	endpoints!: ENDPOINT[];
	eventTags: string[] = [];
	apps!: APP[];
	sources!: SOURCE[];
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

	projectType?: 'incoming' | 'outgoing';
	isLoadingForm = true;
	isLoadingPortalProject = false;
	token: string = this.route.snapshot.queryParams.token;

	configurations = [{ uid: 'filter_config', name: 'Event Filter', show: false }];
	createdSubscription = false;
	private rbacService = inject(RbacService);
	showFilterDialog = false;
	showTransformDialog = false;
	sourceURL!: string;
	subscription!: SUBSCRIPTION;
	currentRoute = window.location.pathname.split('/').reverse()[0];
	eventTypes: EVENT_TYPE[] = [];
	selectedEventTypes: string[] = [];
	filters: FILTER[] = [];
	selectedEventType: string = '';

	constructor(
		private formBuilder: FormBuilder,
		private privateService: PrivateService,
		private createSubscriptionService: CreateSubscriptionService,
		private route: ActivatedRoute,
		private router: Router,
		public licenseService: LicensesService,
		private filterService: FilterService,
		private cdr: ChangeDetectorRef
	) {}

	// Getter for the eventTypes FormGroup
	get eventTypesFormGroup(): FormGroup {
		return this.subscriptionForm.get('eventTypes') as FormGroup;
	}

	async ngOnInit() {
		this.isLoadingForm = true;

		await this.getEventTypes();

		this.projectType = this.token ? 'outgoing' : this.privateService.getProjectDetails?.type;

		if (!this.subscriptionId) this.subscriptionId = this.route.snapshot.params.id || this.route.snapshot.queryParams.id;

		if (this.isPortal === 'true' || this.token)
			this.subscriptionForm.patchValue({
				endpoint_id: this.endpointId
			});

		if (this.isPortal === 'true' && !this.endpointId) this.getEndpoints();

		if (this.isPortal !== 'true' && this.showAction === 'true') await Promise.all([this.getEndpoints(), this.getSources()]);

		if (this.action === 'update' || this.isUpdateAction) await this.getSubscriptionDetails();
		else {
			// Initialize selectedEventTypes with valid event types if empty
			if (this.selectedEventTypes.length === 0 && this.eventTypes.length > 0) {
				this.selectedEventTypes = [this.eventTypes[0].name];
			}
		}

		// Initialize eventTypes form group if we have selectedEventTypes
		if (this.selectedEventTypes.length > 0) {
			// Create a form group with string keys
			const eventTypesControls: Record<string, any> = {};
			this.selectedEventTypes.forEach((eventType, index) => {
				eventTypesControls[index.toString()] = this.formBuilder.control(eventType);
			});

			// Set the form group values
			this.subscriptionForm.setControl('eventTypes', this.formBuilder.group(eventTypesControls));
		}

		// For new subscriptions with outgoing project type, initialize with at least one event type
		if (this.action === 'create' && this.projectType === 'outgoing' && this.selectedEventTypes.length === 0 && this.eventTypes.length > 0) {
			this.selectedEventTypes = [this.eventTypes[0].name];
			const eventTypesControls: Record<string, any> = {
				'0': this.formBuilder.control(this.eventTypes[0].name)
			};
			this.subscriptionForm.setControl('eventTypes', this.formBuilder.group(eventTypesControls));
			this.toggleConfigForm('events', true);
		}

		// If we have selected event types, make sure to show the event types section
		if (this.selectedEventTypes.length > 0) {
			this.toggleConfigForm('events', true);
		}

		this.isLoadingForm = false;

		// add required validation on source input for incoming projects
		if (this.projectType === 'incoming') {
			this.subscriptionForm.get('source_id')?.addValidators(Validators.required);
			this.subscriptionForm.get('source_id')?.updateValueAndValidity();
			this.configurations.push({ uid: 'tranform_config', name: 'Transform', show: false });
		} else {
			this.configurations.push({ uid: 'events', name: 'Event Types', show: false });
		}

		if (this.configSetting) this.toggleConfigForm(this.configSetting, true);
		if (!(await this.rbacService.userCanAccess('Subscriptions|MANAGE'))) this.subscriptionForm.disable();

		this.toggleConfigForm('events', true);
	}

	ngAfterViewInit() {
		// Set up a listener for when the component view is stable
		setTimeout(() => {
			this.initializeSelectComponents();
		}, 300);
	}

	// Initialize all convoy-select components manually
	initializeSelectComponents() {
		if (!this.eventTypeSelects || this.eventTypeSelects.length === 0) {
			// Try again after a longer delay if components haven't rendered yet
			setTimeout(() => this.initializeSelectComponents(), 500);
			return;
		}

		this.eventTypeSelects.forEach((select, index) => {
			if (index < this.selectedEventTypes.length) {
				// Get the event type value from the array
				const eventTypeValue = this.selectedEventTypes[index];

				// Handle string values by default
				let selectedValue: string | EVENT_TYPE = eventTypeValue;

				// If it's a string, try to find the corresponding object
				if (typeof eventTypeValue === 'string') {
					const matchingObj = this.eventTypes.find(et => et.name === eventTypeValue);
					if (matchingObj) {
						selectedValue = matchingObj;
					}
				}

				// Update the select component
				if (select) {
					// Set the form control value
					const control = this.eventTypesFormGroup.get(index.toString());
					if (control) {
						const formValue = typeof selectedValue === 'string' ? selectedValue : (selectedValue as EVENT_TYPE).name;
						control.setValue(formValue);
					}

					// Directly set the value and selected value on the component
					if (typeof selectedValue === 'string') {
						select.value = selectedValue;
						select.selectedValue = selectedValue;
						select.writeValue(selectedValue);
					} else {
						select.value = (selectedValue as EVENT_TYPE).name;
						select.selectedValue = selectedValue;
						select.writeValue((selectedValue as EVENT_TYPE).name);
					}
				}
			}
		});

		// Force change detection
		this.cdr.detectChanges();
	}

	toggleConfig(configValue: string) {
		this.action === 'view' ? this.router.navigate(['/projects/' + this.privateService.getProjectDetails?.uid + '/subscriptions/' + this.subscriptionId], { queryParams: { configSetting: configValue } }) : this.toggleConfigForm(configValue);
	}

	deleteFilterAndToggleConfigForm(configValue: string, value?: boolean) {
		// other cleanup ops

		this.toggleConfigForm(configValue, value);
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
				this.selectedEventTypes = [...this.eventTags];

				// Create a form group with string keys
				const eventTypesControls: Record<string, any> = {};
				this.selectedEventTypes.forEach((eventType, index) => {
					eventTypesControls[index.toString()] = this.formBuilder.control(eventType);
				});

				// Set the form group values
				this.subscriptionForm.setControl('eventTypes', this.formBuilder.group(eventTypesControls));

				// Show the event types section
				this.toggleConfigForm('events', true);

				// Initialize select components after a short delay
				setTimeout(() => {
					this.initializeSelectComponents();
				}, 300);
			} else {
				this.eventTags = [];
			}
			const filterConfig = response.data.filter_config?.filter;

			if (this.action === 'update' && (Object.keys(filterConfig.body).length > 0 || Object.keys(filterConfig.headers).length > 0)) {
				this.configurations.forEach(config => {
					if (config.uid === 'filter_config') config.show = true;
				});
			}

			if (this.token) this.projectType = 'outgoing';

			if (response.data?.function) this.toggleConfigForm('tranform_config');

			// Get filters for this subscription
			if (this.subscriptionId) {
				await this.getFilters();
			}

			// Manually trigger change detection
			this.cdr.detectChanges();

			return;
		} catch (error) {
			console.error('Error loading subscription details:', error);
			return error;
		}
	}

	async getEndpoints(searchString?: string) {
		try {
			const response = await this.privateService.getEndpoints({ q: searchString });
			this.endpoints = response.data.content;
			return;
		} catch (error) {
			return error;
		}
	}

	async getSources(searchString?: string) {
		if (this.privateService.getProjectDetails?.type === 'outgoing' || this.token) return;

		try {
			const sourcesResponse = await this.privateService.getSources({ q: searchString });
			this.sources = sourcesResponse.data.content;
			return;
		} catch (error) {
			return;
		}
	}

	async getEventTypes() {
		if (this.privateService.getProjectDetails?.type === 'incoming') return;

		try {
			const response = await this.privateService.getEventTypes();

			const { event_types } = response.data;
			this.eventTypes = event_types.filter((type: EVENT_TYPE) => !type.deprecated_at);

			return;
		} catch (error) {
			console.error('Error loading event types:', error);
			return;
		}
	}

	async onCreateSource(newSource: SOURCE) {
		this.subscriptionForm.patchValue({ source_id: newSource.uid });
		this.sourceURL = newSource.url;
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
		// Validate form before submitting
		if (this.subscriptionForm.invalid) {
			console.error('Form is invalid:', this.subscriptionForm.errors);
			return;
		}

		// Check if event types are required and set
		if (this.projectType === 'outgoing' && this.showConfig('events') && Object.keys(this.eventTypesFormGroup.controls).length === 0) {
			console.error('Event types are required for outgoing projects');
			return;
		}

		this.toggleFormsLoaders(true);

		// If no event types are selected, use the wildcard
		if (this.selectedEventTypes.length === 0) {
			this.selectedEventTypes = ['*'];
		}

		this.subscriptionForm.patchValue({
			filter_config: {
				event_types: this.selectedEventTypes
			}
		});

		await this.runSubscriptionValidation();

		if (this.subscriptionForm.get('name')?.invalid || this.subscriptionForm.get('filter_config')?.invalid) {
			this.toggleFormsLoaders(false);
			this.subscriptionForm.markAllAsTouched();
			return;
		}

		if (this.createEndpointForm && !this.createEndpointForm.endpointCreated) await this.createEndpointForm.saveEndpoint();
		if (this.createSourceForm && !this.createSourceForm.sourceCreated) await this.createSourceForm.saveSource();

		if (!this.showAction && this.endpoints.length) this.subscriptionForm.patchValue({ name: this.endpoints?.find(endpoint => endpoint.uid == this.subscriptionForm.value.endpoint_id)?.name + ' Subscription' });

		// check subscription form validation
		if (this.subscriptionForm.invalid) {
			this.isCreatingSubscription = false;
			return this.subscriptionForm.markAllAsTouched();
		}

		let deletedFormFilter = false;
		if (!this.showConfig('filter_config')) {
			const filterGroup = this.subscriptionForm.get('filter_config.filter') as FormGroup;
			if (filterGroup) {
				filterGroup.patchValue({
					headers: {},
					body: {}
				});
				deletedFormFilter = true;
			}
		}

		// check if configs are added, else delete the properties
		const subscriptionData = structuredClone(this.subscriptionForm.value);

		// If we have event types, include them in the request
		if (this.selectedEventTypes.length > 0) {
			// Extract values from the form group
			const eventTypesValues = Object.values(this.eventTypesFormGroup.value);

			// Update payload with event types
			if (this.projectType === 'outgoing') {
				subscriptionData.filter_config = {
					...(subscriptionData.filter_config || {}),
					event_types: this.selectedEventTypes // Use the selectedEventTypes array directly
				};
			}
		}

		// create subscription
		try {
			let response;
			if (this.action === 'update' || this.isUpdateAction) {
				response = await this.createSubscriptionService.updateSubscription({ data: subscriptionData, id: this.subscriptionId });
			} else {
				response = await this.createSubscriptionService.createSubscription(subscriptionData);
				this.subscriptionId = response.data.uid;
			}

			// Save filters after subscription is created/updated
			if (this.filters.length > 0) {
				const filterRequests: FILTER_CREATE_REQUEST[] = this.filters.map(filter => ({
					subscription_id: this.subscriptionId,
					event_type: filter.event_type,
					headers: filter.headers,
					body: filter.body,
					raw_headers: filter.raw_headers,
					raw_body: filter.raw_body
				}));

				await this.filterService.createFilters(this.subscriptionId, filterRequests);
			}

			this.subscription = response.data;
			if (setup) await this.privateService.getProjectStat({ refresh: true });
			this.privateService.getSubscriptions();
			localStorage.removeItem('FUNCTION');
			this.createdSubscription = true;
			if (deletedFormFilter) {
				localStorage.setItem('DELETE_FILTER_SETUP', 'true');
			}
			if (this.sourceURL) return this.sourceURLDialog.nativeElement.showModal();
			this.onAction.emit({ data: this.subscription, action: this.action == 'update' ? 'update' : 'create' });
		} catch (error) {
			this.createdSubscription = false;
			this.isCreatingSubscription = false;
		}
	}

	goToSubsriptionsPage() {
		this.router.navigateByUrl('/projects/' + this.privateService.getProjectDetails?.uid + '/subscriptions');
	}

	setupFilter() {
		let deleteFilter = localStorage.getItem('DELETE_FILTER_SETUP');
		if (deleteFilter) {
			this.subscriptionForm.get('filter_config.filter.headers')?.patchValue({});
			this.subscriptionForm.get('filter_config.filter.body')?.patchValue({});
			localStorage.removeItem('DELETE_FILTER_SETUP');
		}
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

	get isUpdateAction(): boolean {
		return this.subscriptionId && this.subscriptionId !== 'new' && this.currentRoute !== 'setup';
	}

	async getFilters() {
		try {
			const response = await this.filterService.getFilters(this.subscriptionId);
			this.filters = response.data.content || [];
			return;
		} catch (error) {
			return error;
		}
	}

	openFilterDialog(eventType?: string) {
		this.selectedEventType = eventType || '';
		this.showFilterDialog = true;
	}

	onSaveFilter(filters: FILTER[]) {
		this.filters = filters;
	}

	addEventType() {
		// Check if there are available event types
		if (this.eventTypes.length > 0) {
			// Get current index
			const index = this.selectedEventTypes.length;
			const indexStr = index.toString();

			// Create an updated controls object
			const updatedControls: Record<string, any> = { ...this.eventTypesFormGroup.value };
			updatedControls[indexStr] = this.eventTypes[0].name;

			// Update the form group
			this.eventTypesFormGroup.addControl(indexStr, this.formBuilder.control(this.eventTypes[0].name));

			// Update the selectedEventTypes array to stay in sync
			this.selectedEventTypes.push(this.eventTypes[0].name);

			// Manually trigger change detection
			this.cdr.detectChanges();

			// Initialize the new select component
			setTimeout(() => {
				this.initializeSelectComponents();
			}, 50);
		} else {
			console.warn('No event types available to add.');
		}
	}

	removeEventType(index: number) {
		// Get the event type being removed
		const eventType = this.selectedEventTypes[index];
		const indexStr = index.toString();

		// Remove from form group
		this.eventTypesFormGroup.removeControl(indexStr);

		// Remove from selectedEventTypes array to keep them in sync
		this.selectedEventTypes.splice(index, 1);

		// Reindex the remaining controls
		const updatedControls: Record<string, any> = {};
		this.selectedEventTypes.forEach((evType, i) => {
			updatedControls[i.toString()] = this.formBuilder.control(evType);
		});

		// Recreate the form group with the updated controls
		this.subscriptionForm.setControl('eventTypes', this.formBuilder.group(updatedControls));

		// Also remove any filters for this event type
		this.filters = this.filters.filter(filter => filter.event_type !== eventType);

		// Initialize the remaining select components
		setTimeout(() => {
			this.initializeSelectComponents();
		}, 50);
	}

	updateEventType(index: number, eventType: string | EVENT_TYPE) {
		// Update the event type in the form group
		const newEventType = typeof eventType === 'string' ? eventType : eventType.name;

		// Update the form control
		this.eventTypesFormGroup.get(index.toString())?.setValue(newEventType);

		// Update the selectedEventTypes array to stay in sync
		const oldEventType = this.selectedEventTypes[index];
		this.selectedEventTypes[index] = String(newEventType);

		// Update any filters for this event type
		const filterIndex = this.filters.findIndex(filter => filter.event_type === oldEventType);
		if (filterIndex >= 0) {
			this.filters[filterIndex].event_type = String(newEventType);
		}

		// Force UI update with change detection
		this.cdr.detectChanges();

		// Re-initialize select components after a short delay
		setTimeout(() => {
			// Force a component redraw by recreating the event types array
			this.selectedEventTypes = [...this.selectedEventTypes];
			this.initializeSelectComponents();
		}, 50);
	}

	updateSelectedEventType(index: number, eventType: string | EVENT_TYPE) {
		// Handle both string and EVENT_TYPE objects
		const newEventType = typeof eventType === 'string' ? eventType : eventType.name;
		this.selectedEventTypes[index] = newEventType;
	}
}
