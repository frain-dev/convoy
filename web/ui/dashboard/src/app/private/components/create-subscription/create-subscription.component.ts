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
export class CreateSubscriptionComponent implements OnInit {
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
	filtersMap: Map<string, FILTER> = new Map<string, FILTER>();
	selectedEventType: string = '';
	selectedIndex: number = 0;

	// Flag to prevent infinite recursion in toggleEventType
	private _isTogglingEventType = false;

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

		if (this.isPortal === 'true' && !this.endpointId) await this.getEndpoints();

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

		// For new subscriptions with outgoing project type, initialize with at least one event type
		if (this.action === 'create' && this.projectType === 'outgoing' && this.selectedEventTypes.length === 0 && this.eventTypes.length > 0) {
			// Default to the '*' (wildcard) event type for new subscriptions if available
			const wildcardExists = this.eventTypes.some(type => type.name === '*');

			if (wildcardExists) {
				console.log('Initializing new subscription with wildcard (*) event type');
				this.selectedEventTypes = ['*'];
				const eventTypesControls: Record<string, any> = {
					'0': this.formBuilder.control('*')
				};
				this.subscriptionForm.setControl('eventTypes', this.formBuilder.group(eventTypesControls));
			} else {
				// Fall back to first event type if wildcard doesn't exist
				console.log(`Initializing new subscription with first available event type: ${this.eventTypes[0].name}`);
				this.selectedEventTypes = [this.eventTypes[0].name];
				const eventTypesControls: Record<string, any> = {
					'0': this.formBuilder.control(this.eventTypes[0].name)
				};
				this.subscriptionForm.setControl('eventTypes', this.formBuilder.group(eventTypesControls));
			}

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
				endpoint_id: matchingEndpoint?.uid || endpointId
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

			// Handle event types with wildcard support
			if (response.data.filter_config?.event_types) {
				this.eventTags = response.data.filter_config?.event_types;

				// Check if wildcard is present
				const hasWildcard = this.eventTags.includes('*');

				// If wildcard is present, only use the wildcard and ignore other event types
				if (hasWildcard) {
					console.log('Wildcard (*) event type detected in subscription, ignoring other event types');
					this.selectedEventTypes = ['*'];
				} else {
					// Otherwise, use all the event types
					this.selectedEventTypes = [...this.eventTags];
				}

				// Create a form group with string keys
				const eventTypesControls: Record<string, any> = {};
				this.selectedEventTypes.forEach((eventType, index) => {
					eventTypesControls[index.toString()] = this.formBuilder.control(eventType);
				});

				// Set the form group values
				this.subscriptionForm.setControl('eventTypes', this.formBuilder.group(eventTypesControls));

				// Also update the filters to match the selected event types
				// Remove any filters for event types that are no longer selected
				this.filters = this.filters.filter(filter => this.selectedEventTypes.includes(filter.event_type));

				// Sync with the filtersMap
				this.filtersMap.clear();
				this.filters.forEach(filter => {
					this.filtersMap.set(filter.event_type, { ...filter });
				});

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
			this.eventTypes = response.data.filter((type: EVENT_TYPE) => !type.deprecated_at);
		} catch (error) {
			console.error('Error loading event types:', error);
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
		// create the endpoint and source first
		this.toggleFormsLoaders(true);
		if (this.createEndpointForm && !this.createEndpointForm.endpointCreated) await this.createEndpointForm.saveEndpoint();
		if (this.createSourceForm && !this.createSourceForm.sourceCreated) await this.createSourceForm.saveSource();

		// Validate form before submitting
		if (this.subscriptionForm.invalid) {
			console.log(this.subscriptionForm);
			console.error('Form is invalid:', this.subscriptionForm.errors);
			return;
		}

		// Check if event types are required and set
		if (this.projectType === 'outgoing' && this.showConfig('events') && Object.keys(this.eventTypesFormGroup.controls).length === 0) {
			console.error('Event types are required for outgoing projects');
			return;
		}

		this.toggleFormsLoaders(true);

		// STEP 1: Handle event type selection and ensure mutual exclusivity with wildcard (*)

		// If no event types are selected, use the wildcard
		if (this.selectedEventTypes.length === 0) {
			console.log('No event types selected, using wildcard (*)');
			this.selectedEventTypes = ['*'];
		}

		// Enforce mutual exclusivity between wildcard (*) and specific event types
		const hasWildcard = this.selectedEventTypes.includes('*');
		if (hasWildcard && this.selectedEventTypes.length > 1) {
			console.log('Both wildcard (*) and specific event types selected. Using only wildcard.');
			// If wildcard is selected, ignore other event types
			this.selectedEventTypes = ['*'];

			// Also update the filtersMap to include only the wildcard
			const wildcardFilter = this.filtersMap.get('*');
			this.filtersMap.clear();
			if (wildcardFilter) {
				this.filtersMap.set('*', wildcardFilter);
			} else {
				this.filtersMap.set('*', {
					uid: '', // Will be assigned by backend
					subscription_id: this.subscriptionId,
					event_type: '*',
					headers: {},
					body: {},
					is_new: true
				});
			}

			// Sync with filters array
			this._syncFiltersArrayWithMap();
		}

		// STEP 2: Update the form with the final event types selection
		this.subscriptionForm.patchValue({
			filter_config: {
				event_types: this.selectedEventTypes
			}
		});

		// STEP 3: Validate the subscription
		await this.runSubscriptionValidation();

		// Clean up the duplicate code above and consolidate event type handling
		if (this.subscriptionForm.get('name')?.invalid || this.subscriptionForm.get('filter_config')?.invalid) {
			this.toggleFormsLoaders(false);
			this.subscriptionForm.markAllAsTouched();
			return;
		}

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

			// Save filters after subscription is created/updated (using Map implementation)
			try {
				// Get the existing filters once to avoid multiple API calls
				const existingFiltersResponse = await this.filterService.getFilters(this.subscriptionId);
				const existingFiltersContent = existingFiltersResponse.data || [];

				// Create a map of existing filters by event type for easy lookup
				const existingFiltersByEventType: { [key: string]: any } = {};
				existingFiltersContent.forEach((filter: any) => {
					existingFiltersByEventType[filter.event_type] = filter;
				});

				// Process filters in the map
				const filtersToUpdate: any[] = [];
				const filtersToCreate: any[] = [];

				// Process each filter in the map
				this.filtersMap.forEach((filter, eventType) => {
					// Check if this filter needs to be updated or created
					const existingFilter = existingFiltersByEventType[eventType];

					if (existingFilter) {
						// This is an existing filter that needs to be updated
						const updatePayload: any = {
							uid: existingFilter.uid,
							headers: filter.headers || {},
							body: filter.body || {}
						};

						// Only include event_type if it's different from the existing one
						if (eventType !== existingFilter.event_type) {
							updatePayload.event_type = eventType;
						}

						filtersToUpdate.push(updatePayload);
					} else {
						// This is a new filter that needs to be created
						filtersToCreate.push({
							subscription_id: this.subscriptionId,
							event_type: eventType,
							headers: filter.headers || {},
							body: filter.body || {},
							raw_headers: filter.raw_headers || {},
							raw_body: filter.raw_body || {}
						});
					}
				});

				console.log('Filters to create:', filtersToCreate);
				console.log('Filters to update:', filtersToUpdate);

				// Create new filters in bulk if needed
				if (filtersToCreate.length > 0) {
					await this.filterService.createFilters(this.subscriptionId, filtersToCreate);
				}

				// Update existing filters in bulk if needed
				if (filtersToUpdate.length > 0) {
					try {
						await this.filterService.bulkUpdateFilters(this.subscriptionId, filtersToUpdate);
					} catch (error) {
						console.error('Error calling bulkUpdateFilters:', error);
					}
				}
			} catch (error) {
				console.error('Error saving filters:', error);
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
		if (!this.selectedEventType) {
			console.error('No event type selected for filter');
			return;
		}

		// Get the existing filter from the map or create a default object
		const existingFilter = this.filtersMap.get(this.selectedEventType) || {
			uid: '', // Will be assigned by backend
			subscription_id: this.subscriptionId,
			event_type: this.selectedEventType,
			raw_headers: {},
			raw_body: {},
			created_at: new Date().toISOString(),
			updated_at: new Date().toISOString(),
			is_new: true
		};

		// Update the filter with the new schema data
		const updatedFilter = {
			...existingFilter,
			headers: schema.headerSchema || {},
			body: schema.bodySchema || {},
			// Mark as modified to help with syncing to the backend
			is_modified: true
		};

		// Save the updated filter to the map
		this.filtersMap.set(this.selectedEventType, updatedFilter);

		// Sync with filters array for compatibility
		this._syncFiltersArrayWithMap();

		console.log(`Filter for "${this.selectedEventType}" updated:`, updatedFilter);

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
			this.filters = response.data; // Keep the array for backward compatibility

			// Clear the map and populate it with the filter data from the response
			this.filtersMap.clear();
			response.data.forEach((filter: FILTER) => {
				// Use event_type as the key for the map
				this.filtersMap.set(filter.event_type, { ...filter });
			});
			return response;
		} catch (error) {
			console.error('Error fetching filters:', error);
			throw error;
		}
	}

	openFilterDialog(eventType: string) {
		document.getElementById(this.showAction === 'true' ? 'subscriptionForm' : 'configureProjectForm')?.scroll({ top: 0, behavior: 'smooth' });
		this.selectedEventType = eventType || '';

		// For backward compatibility
		this.selectedIndex = this.filters.findIndex(item => item.event_type === eventType);

		// Ensure we have a filter entry for this event type in the map
		if (!this.filtersMap.has(eventType) && eventType) {
			// Create a new filter entry if it doesn't exist
			this.filtersMap.set(eventType, {
				uid: '', // Will be assigned by backend
				subscription_id: this.subscriptionId,
				event_type: eventType,
				headers: {},
				body: {},
				is_new: true
			});

			// Sync with filters array for compatibility
			this._syncFiltersArrayWithMap();
		}

		this.showFilterDialog = true;
	}

	removeEventType(index: number, isAutoSelect = false) {
		// Get the event type being removed
		const eventType = this.selectedEventTypes[index];
		const isWildcard = eventType === '*';

		// Log the action for debugging
		console.log(`Removing event type: ${eventType}`);

		// Remove from selectedEventTypes array
		this.selectedEventTypes.splice(index, 1);

		// Remove from the filtersMap
		this.filtersMap.delete(eventType);

		// If we just removed the last event type, and it wasn't a wildcard removal
		// as part of selecting another event type, add the wildcard as default
		if (this.selectedEventTypes.length === 0 && !isWildcard && !isAutoSelect) {
			this.toggleEventType('*');
		}

		// Sync with filters array for compatibility
		this._syncFiltersArrayWithMap();

		// Force UI update
		this.cdr.detectChanges();
	}

	updateEventType(index: number, eventType: string | EVENT_TYPE) {
		// Update the event type in the form group
		const newEventType = typeof eventType === 'string' ? eventType : eventType.name;

		// Update the form control
		this.eventTypesFormGroup.get(index.toString())?.setValue(newEventType);

		// Get the old event type from the array
		const oldEventType = this.selectedEventTypes[index];

		// Skip if they're the same
		if (oldEventType === newEventType) {
			return;
		}

		console.log(`Updating event type from "${oldEventType}" to "${newEventType}"`);

		// Get the filter for the old event type
		const oldFilter = this.filtersMap.get(oldEventType);

		// Check if there's already a filter for the new event type
		const hasNewTypeFilter = this.filtersMap.has(newEventType);

		// If we have an old filter, update its event type
		if (oldFilter && !hasNewTypeFilter) {
			// Clone the filter and update its event type
			const updatedFilter = {
				...oldFilter,
				event_type: newEventType,
				// Mark as new if it doesn't have a UID
				is_new: !oldFilter.uid,
				// Mark as modified to help with syncing to the backend
				is_modified: true
			};

			// Remove the old filter and add the updated one
			this.filtersMap.delete(oldEventType);
			this.filtersMap.set(newEventType, updatedFilter);

			console.log(`Filter updated from "${oldEventType}" to "${newEventType}"`, updatedFilter);
		} else if (!hasNewTypeFilter) {
			// If there's no filter for either the old or new event type, create a new one
			this.filtersMap.set(newEventType, {
				uid: '', // Will be assigned by backend
				subscription_id: this.subscriptionId,
				event_type: newEventType,
				headers: {},
				body: {},
				is_new: true
			});

			console.log(`New filter created for "${newEventType}"`);
		}

		// If the old filter exists but there's also already a filter for the new type,
		// just delete the old one as we'll use the existing new one
		if (oldFilter && hasNewTypeFilter) {
			this.filtersMap.delete(oldEventType);
			console.log(`Old filter for "${oldEventType}" deleted as "${newEventType}" already has a filter`);
		}

		// Update the selectedEventTypes array
		this.selectedEventTypes[index] = newEventType;

		// Sync with filters array for compatibility
		this._syncFiltersArrayWithMap();

		// Force UI update
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

	// Toggle an event type on/off with special handling for wildcard (*)
	toggleEventType(eventTypeName: string, isAutoSelect = false): void {
		// Prevent toggling if subscription is being created/updated
		if (this.isCreatingSubscription) {
			console.warn('Cannot toggle event type while subscription is being saved');
			return;
		}

		// Prevent recursive calls
		if (this._isTogglingEventType) {
			console.warn('Already toggling an event type, skipping');
			return;
		}

		try {
			this._isTogglingEventType = true;

			const index = this.selectedEventTypes.indexOf(eventTypeName);
			const isWildcard = eventTypeName === '*';

			if (index !== -1) {
				// If already selected, just remove it
				this.removeEventType(index, isAutoSelect);
				return;
			}

			// If selecting the wildcard (*) event type, remove all other event types
			if (isWildcard) {
				console.log('Selecting wildcard (*) event type - removing all other event types');

				// Store all event types that need to be removed
				const eventTypesToRemove = [...this.selectedEventTypes];

				// Clear all selected event types and their filters
				eventTypesToRemove.forEach(type => {
					const typeIndex = this.selectedEventTypes.indexOf(type);
					if (typeIndex !== -1) {
						this.removeEventType(typeIndex, true);
					}
				});
			}
			// If selecting a specific event type, remove the wildcard (*) if it's selected
			else if (this.selectedEventTypes.includes('*')) {
				console.log(`Selecting specific event type '${eventTypeName}' - removing wildcard (*) event type`);
				const wildcardIndex = this.selectedEventTypes.indexOf('*');
				this.removeEventType(wildcardIndex, true);
			}

			// Add the newly selected event type
			this.selectedEventTypes.push(eventTypeName);

			// Add a filter for this event type to the map
			this.filtersMap.set(eventTypeName, {
				uid: '', // Will be assigned by backend
				subscription_id: this.subscriptionId,
				event_type: eventTypeName,
				headers: {},
				is_new: true,
				body: {}
			});

			// Sync with filters array for compatibility
			this._syncFiltersArrayWithMap();

			// Force UI update
			this.cdr.detectChanges();
		} finally {
			this._isTogglingEventType = false;
		}
	}

	validEventTypes(): EVENT_TYPE[] {
		return this.eventTypes.filter(type => type.name !== '*')
	}

	// Helper method to sync filters array with filtersMap
	private _syncFiltersArrayWithMap(): void {
		// Convert the map values to an array and assign to filters
		this.filters = Array.from(this.filtersMap.values());
	}

	protected readonly Number = Number;
}
