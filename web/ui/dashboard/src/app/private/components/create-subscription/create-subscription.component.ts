import { AfterViewInit, ChangeDetectorRef, Component, ElementRef, EventEmitter, inject, Input, OnInit, Output, QueryList, ViewChild, ViewChildren } from '@angular/core';
import { FormBuilder, FormGroup, Validators } from '@angular/forms';
import { ActivatedRoute, Router } from '@angular/router';
import { APP, ENDPOINT } from 'src/app/models/endpoint.model';
import { SOURCE } from 'src/app/models/source.model';
import { PrivateService } from '../../private.service';
import { CreateEndpointComponent } from '../create-endpoint/create-endpoint.component';
import { CreateSourceComponent } from '../create-source/create-source.component';
import { CreateSubscriptionService } from './create-subscription.service';
import { RbacService } from 'src/app/services/rbac/rbac.service';
import { SUBSCRIPTION, SUBSCRIPTION_CONFIG } from 'src/app/models/subscription';
import { LicensesService } from 'src/app/services/licenses/licenses.service';
import { EVENT_TYPE } from 'src/app/models/event.model';
import { FILTER } from 'src/app/models/filter.model';
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

	configurations: SUBSCRIPTION_CONFIG[] = [];
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
	selectedIndex: number = 0;

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
		// Only for update mode - initialize select components
		if (this.action === 'update' || this.isUpdateAction) {
			// Wait for endpoint data to be loaded
			setTimeout(async () => {
				// Check if we have a subscription with endpoint metadata
				const endpointId = this.subscription?.endpoint_metadata?.uid;

				// If we don't have endpoints yet but we have an endpoint ID
				if ((!this.endpoints || this.endpoints.length === 0) && endpointId) {
					await this.getEndpoints();

					// Find matching endpoint and set it
					if (this.endpoints && this.endpoints.length > 0 && endpointId) {
						const matchingEndpoint = this.endpoints.find(endpoint => endpoint.uid === endpointId);

						if (matchingEndpoint) {
							this.subscriptionForm.patchValue({ endpoint_id: matchingEndpoint });

							// Force change detection
							this.cdr.detectChanges();
						}
					}
				}
			}, 500);
		}
	}

	toggleConfig(configValue: string) {
		this.action === 'view' ? this.router.navigate(['/projects/' + this.privateService.getProjectDetails?.uid + '/subscriptions/' + this.subscriptionId], { queryParams: { configSetting: configValue } }) : this.toggleConfigForm(configValue);
	}

	toggleConfigForm(configValue: string, value?: boolean) {
		this.configurations?.forEach(config => {
			if (config.uid === configValue) config.show = value ? value : !config.show;
		});
	}

	showConfig(configValue: string): boolean {
		return this.configurations.find(config => config?.uid === configValue)?.show || false;
	}

	async getSubscriptionDetails() {
		try {
			const response = await this.createSubscriptionService.getSubscriptionDetail(this.subscriptionId);

			// Store the subscription data
			this.subscription = response.data;

			// First, load the endpoints if not already loaded
			if (!this.endpoints || this.endpoints.length === 0) {
				await this.getEndpoints();
			}

			// Similarly, load sources if not already loaded and if it's an incoming project
			if ((!this.sources || this.sources.length === 0) && this.projectType === 'incoming' && !this.token) {
				await this.getSources();
			}

			// Find the matching endpoint from options directly
			const endpointId = response.data?.endpoint_metadata?.uid;
			let matchingEndpoint = null;

			if (endpointId && this.endpoints && this.endpoints.length > 0) {
				matchingEndpoint = this.endpoints.find(e => e.uid === endpointId);
			}

			// Find the matching source from options directly
			const sourceId = response.data?.source_metadata?.uid;
			let matchingSource = null;

			if (sourceId && this.sources && this.sources.length > 0) {
				matchingSource = this.sources.find(s => s.uid === sourceId);
			}

			// Set the form values
			this.subscriptionForm.patchValue({
				...response.data,
				// For sources and endpoints, use the full object if found or just the ID
				source_id: matchingSource || sourceId,
				endpoint_id: matchingEndpoint || endpointId
			});

			// Handle event types
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
			} else {
				this.eventTags = [];
			}

			// Get filter data
			await this.getFilters();

			if (this.token) this.projectType = 'outgoing';

			if (response.data?.function) this.toggleConfigForm('tranform_config');

			// Manually trigger change detection
			this.cdr.detectChanges();

			return;
		} catch (error) {
			console.error('Error getting subscription details:', error);
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
			const fields = configFields[config?.uid];
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

		// Since we're using per-event-type filters, we don't need the global filter anymore
		// Reset the filter config to an empty object
		const filterGroup = this.subscriptionForm.get('filter_config.filter') as FormGroup;
		if (filterGroup) {
			filterGroup.patchValue({
				headers: {},
				body: {}
			});
		}

		// Get the current form values
		const subscriptionData = structuredClone(this.subscriptionForm.value);

		// ALWAYS convert endpoint_id to UID string
		// This is essential for both for the API call and to prevent objects being sent
		if (subscriptionData.endpoint_id) {
			if (typeof subscriptionData.endpoint_id === 'object') {
				if (subscriptionData.endpoint_id.uid) {
					subscriptionData.endpoint_id = subscriptionData.endpoint_id.uid;
				} else {
					// Try other possible properties that might contain the ID
					const possibleIdFields = ['id', '_id', 'ID', 'value'];
					for (const field of possibleIdFields) {
						if (subscriptionData.endpoint_id[field]) {
							subscriptionData.endpoint_id = subscriptionData.endpoint_id[field];
							break;
						}
					}
				}
			} else if (typeof subscriptionData.endpoint_id !== 'string') {
				console.error('Unexpected endpoint_id type:', typeof subscriptionData.endpoint_id);
				// Convert to string as a fallback
				subscriptionData.endpoint_id = String(subscriptionData.endpoint_id);
			}
		}

		// Similarly, ensure source_id is a string if present
		if (subscriptionData.source_id && typeof subscriptionData.source_id === 'object' && subscriptionData.source_id.uid) {
			subscriptionData.source_id = subscriptionData.source_id.uid;
		}

		// If we have event types, include them in the request
		if (this.selectedEventTypes.length > 0) {
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
				try {
					// Get the existing filters once to avoid multiple API calls
					const existingFiltersResponse = await this.filterService.getFilters(this.subscriptionId);
					const existingFiltersContent = existingFiltersResponse.data || [];

					// Create a map of existing filters by event type for easy lookup
					const existingFiltersByEventType: { [key: string]: any } = {};
					existingFiltersContent.forEach((filter: any) => {
						existingFiltersByEventType[filter.event_type] = filter;
					});

					// Process filters to update - filters with UIDs
					const filtersToUpdate = this.filters
						.filter(filter => {
							// Check if filter has a UID or if there's an existing filter with the same event type
							return !!filter.uid || existingFiltersByEventType[filter.event_type];
						})
						.map(filter => {
							// If filter has no UID but there's a matching event type, use that existing filter's UID
							const matchingFilter = filter.uid ? existingFiltersContent.find((f: any) => f.uid === filter.uid) : existingFiltersByEventType[filter.event_type];

							// Only include event_type if it's actually changed
							const updatePayload: any = {
								uid: filter.uid || (matchingFilter ? matchingFilter.uid : ''),
								headers: filter.headers || {},
								body: filter.body || {}
							};

							// Only include event_type if it's different from the existing one
							if (matchingFilter && filter.event_type !== matchingFilter.event_type) {
								updatePayload.event_type = filter.event_type;
							}

							return updatePayload;
						});

					// Extract filters to create (those without UIDs and no matching event type)
					const filtersToCreate = this.filters
						.filter(filter => {
							// Only create if no UID and no existing filter with same event type
							return !filter.uid && !existingFiltersByEventType[filter.event_type];
						})
						.map(filter => ({
							subscription_id: this.subscriptionId,
							event_type: filter.event_type,
							headers: filter.headers || {},
							body: filter.body || {},
							raw_headers: filter.raw_headers || {},
							raw_body: filter.raw_body || {}
						}));

					console.log('Filters to create:', filtersToCreate);
					console.log('Filters to update:', filtersToUpdate);

					// Create new filters in bulk if needed
					if (filtersToCreate.length > 0) {
						await this.filterService.createFilters(this.subscriptionId, filtersToCreate);
					}

					// Update existing filters in bulk if needed
					if (filtersToUpdate.length > 0) {
						try {
							const updateResponse = await this.filterService.bulkUpdateFilters(this.subscriptionId, filtersToUpdate);
						} catch (error) {
							console.error('Error calling bulkUpdateFilters:', error);
						}
					}
				} catch (error) {
					console.error('Error saving filters:', error);
				}
			}

			this.subscription = response.data;
			if (setup) await this.privateService.getProjectStat({ refresh: true });
			this.privateService.getSubscriptions();
			localStorage.removeItem('FUNCTION');
			this.createdSubscription = true;

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

	setupTransformDialog() {
		document.getElementById(this.showAction === 'true' ? 'subscriptionForm' : 'configureProjectForm')?.scroll({ top: 0, behavior: 'smooth' });
		this.showTransformDialog = true;
	}

	onSaveFilter(schema: any) {
		// This is called when a filter is created or updated for a specific event type
		// It updates the filter in the filter array
		if (!this.selectedEventType) {
			console.error('No event type selected for filter');
			return;
		}

		// Find or create a filter for this event type
		const existingFilterIndex = this.filters.findIndex(filter => filter.event_type === this.selectedEventType);
		console.log('onSaveFilter:', this.selectedEventType);
		console.log('existingFilterIndex:', existingFilterIndex);

		if (existingFilterIndex >= 0) {
			// Update existing filter
			this.filters[existingFilterIndex] = {
				...this.filters[existingFilterIndex],
				headers: schema.headerSchema || {},
				body: schema.bodySchema || {}
			};
			console.log('updated existing filters');
		} else {
			console.log('created new filter');
			// Create new filter
			this.filters.push({
				uid: '', // Will be assigned by backend
				subscription_id: this.subscriptionId,
				event_type: this.selectedEventType,
				headers: schema.headerSchema || {},
				body: schema.bodySchema || {},
				raw_headers: {},
				raw_body: {},
				created_at: new Date().toISOString(),
				updated_at: new Date().toISOString()
			});
		}

		// Close the filter dialog
		this.showFilterDialog = false;
	}

	getFunction(subscriptionFunction: any) {
		if (subscriptionFunction) this.subscriptionForm.get('function')?.patchValue(subscriptionFunction);
		this.showTransformDialog = false;
	}

	get shouldShowBorder(): number {
		return this.configurations.filter(config => config?.show).length;
	}

	get isUpdateAction(): boolean {
		return this.subscriptionId && this.subscriptionId !== 'new' && this.currentRoute !== 'setup';
	}

	async getFilters(): Promise<any> {
		try {
			const response = await this.filterService.getFilters(this.subscriptionId);
			this.filters = response.data;
		} catch (error) {
			throw error;
		}
	}

	openFilterDialog(eventType: string, index: number) {
		document.getElementById(this.showAction === 'true' ? 'subscriptionForm' : 'configureProjectForm')?.scroll({ top: 0, behavior: 'smooth' });
		this.selectedEventType = eventType || '';
		this.selectedIndex = this.filters.findIndex(item => item.event_type === eventType);
		this.showFilterDialog = true;
	}

	removeEventType(index: number) {
		// Get the event type being removed
		const eventType = this.selectedEventTypes[index];

		// Remove from selectedEventTypes array
		this.selectedEventTypes.splice(index, 1);

		// Also remove any filters for this event type
		this.filters = this.filters.filter(filter => filter.event_type !== eventType);

		// Force UI update
		this.cdr.detectChanges();
	}

	updateEventType(index: number, eventType: string | EVENT_TYPE) {
		console.log('eventType:', eventType);

		// Update the event type in the form group
		const newEventType = typeof eventType === 'string' ? eventType : eventType.name;

		// Update the form control
		this.eventTypesFormGroup.get(index.toString())?.setValue(newEventType);

		// Update the selectedEventTypes array to stay in sync
		const oldEventType = this.selectedEventTypes[index];
		this.selectedEventTypes[index] = String(newEventType);
		console.log('eventType:', oldEventType);

		// First check if a filter for the new event type already exists
		const newEventTypeFilterExists = this.filters.some(filter => filter.event_type === newEventType);

		// Find the index of the filter for the old event type
		const newFilterIndex = this.filters.findIndex(filter => {
			if (filter.event_type === '*' && filter.is_new) {
				return filter.event_type === oldEventType;
			}
			return filter.event_type === newEventType;
		});

		const oldFilterIndex = this.filters.findIndex(filter => filter.event_type === oldEventType);

		console.log('selected filter:', newFilterIndex >= 0 ? this.filters[newFilterIndex] : null);

		const indexToUse = newFilterIndex >= 0 ? newFilterIndex : oldFilterIndex;

		// Create a copy of the filter with the new event type
		const originalFilter = this.filters[indexToUse];

		// Clone the filter and update its event type
		const newFilter = {
			...originalFilter,
			uid: '', // New filter won't have a UID until saved
			event_type: String(newEventType),
			is_new: false
		};

		// remove the item before adding the new one
		this.filters.splice(indexToUse, 1);

		// Add the new filter to the filters array
		this.filters.push(newFilter);

		console.log('updated eventType:', JSON.stringify(this.filters));

		// Force UI update with change detection
		this.cdr.detectChanges();
	}

	updateSelectedEventType(index: number, eventType: string | EVENT_TYPE) {
		// Handle both string and EVENT_TYPE objects
		const newEventType = typeof eventType === 'string' ? eventType : eventType.name;
		this.selectedEventTypes[index] = newEventType;
	}

	// Check if an event type is selected
	isEventTypeSelected(eventTypeName: string): boolean {
		return this.selectedEventTypes.includes(eventTypeName);
	}

	// Toggle an event type on/off
	toggleEventType(eventTypeName: string): void {
		const index = this.selectedEventTypes.indexOf(eventTypeName);

		if (index !== -1) {
			// If already selected, remove it
			this.removeEventType(index);
		} else {
			// If not selected, add it
			this.selectedEventTypes.push(eventTypeName);

			// Add a filter for this event type
			this.filters.push({
				uid: '', // Will be assigned by backend
				subscription_id: this.subscriptionId,
				event_type: eventTypeName,
				headers: {},
				is_new: true,
				body: {}
			});

			// Force UI update
			this.cdr.detectChanges();
		}
	}

	protected readonly Number = Number;
}
