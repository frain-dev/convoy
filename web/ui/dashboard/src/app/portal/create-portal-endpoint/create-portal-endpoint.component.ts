import {
    ChangeDetectorRef,
    Component,
    EventEmitter,
    inject,
    Input,
    OnInit,
    Output, ViewChild,
    ViewEncapsulation
} from '@angular/core';
import { CommonModule, NgOptimizedImage } from '@angular/common';
import {
    ControlContainer,
    FormBuilder,
    FormGroup,
    FormGroupDirective,
    ReactiveFormsModule,
    Validators
} from '@angular/forms';
import { ActivatedRoute } from '@angular/router';

import { GeneralService } from '../../services/general/general.service';
import { FilterService } from '../../private/components/create-subscription/filter.service';
import { RbacService } from '../../services/rbac/rbac.service';
import { LicensesService } from '../../services/licenses/licenses.service';
import { PrivateService } from '../../private/private.service';
import { CreateEndpointService } from '../../private/components/create-endpoint/create-endpoint.service';
import { CreateSubscriptionService } from '../../private/components/create-subscription/create-subscription.service';

import {
    InputDirective,
    InputErrorComponent,
    InputFieldDirective,
    LabelComponent
} from '../../components/input/input.component';
import { ButtonComponent } from '../../components/button/button.component';
import { RadioComponent } from '../../components/radio/radio.component';
import { TooltipComponent } from '../../components/tooltip/tooltip.component';
import { CardComponent } from '../../components/card/card.component';
import { FormLoaderComponent } from '../../components/form-loader/form-loader.component';
import { PermissionDirective } from '../../private/components/permission/permission.directive';
import {
    CreateSubscriptionFilterComponent
} from '../../private/components/create-subscription-filter/create-subscription-filter.component';
import {
    CreatePortalTransformFunctionComponent
} from '../create-portal-transform-function/create-portal-transform-function.component';

import { ENDPOINT, SECRET } from '../../models/endpoint.model';
import { EVENT_TYPE } from '../../models/event.model';
import { FILTER } from '../../models/filter.model';
import { SUBSCRIPTION } from '../../models/subscription';
import { EndpointsService } from '../../private/pages/project/endpoints/endpoints.service';
import { NotificationComponent } from '../../components/notification/notification.component';
import { ConfigButtonComponent } from '../../private/components/config-button/config-button.component';
import { LoaderModule } from '../../private/components/loader/loader.module';
import { TagComponent } from '../../components/tag/tag.component';
import { DialogDirective } from '../../components/dialog/dialog.directive';
import { CopyButtonComponent } from '../../components/copy-button/copy-button.component';

@Component({
	selector: 'convoy-create-portal-endpoint',
	standalone: true,
    imports: [
        CommonModule,
        NgOptimizedImage,
        ReactiveFormsModule,
        InputDirective,
        InputFieldDirective,
        InputErrorComponent,
        LabelComponent,
        ButtonComponent,
        RadioComponent,
        TooltipComponent,
        CardComponent,
        FormLoaderComponent,
        PermissionDirective,
        CreateSubscriptionFilterComponent,
        CreatePortalTransformFunctionComponent,
        NotificationComponent,
        ConfigButtonComponent,
        LoaderModule,
        TagComponent,
        DialogDirective,
        CopyButtonComponent
    ],
	providers: [
		{
			provide: ControlContainer,
			useExisting: FormGroupDirective
		},
		FormGroupDirective,
		FilterService,
		CreateEndpointService,
		CreateSubscriptionService
	],
	templateUrl: './create-portal-endpoint.component.html',
	styleUrls: ['./create-portal-endpoint.component.scss'],
	encapsulation: ViewEncapsulation.None
})
export class CreatePortalEndpointComponent implements OnInit {
	@Input('editMode') editMode = false;
	@Input('showAction') showAction: 'true' | 'false' = 'false';
	@Input('action') action: 'update' | 'create' | 'view' = 'create';
	@Input('type') type: 'in-app' | 'portal' | 'subscription' = 'in-app';
	@Input('subscriptionId') subscriptionId = this.route.snapshot.params.id || this.route.snapshot.queryParams.id;
	@Input('endpointId') endpointUid = this.route.snapshot.params.id;
	@Output() onAction = new EventEmitter<any>();

	// Injected Services
	private rbacService = inject(RbacService);
	private formBuilder = inject(FormBuilder);
	private generalService = inject(GeneralService);
	private createEndpointService = inject(CreateEndpointService);
	private createSubscriptionService = inject(CreateSubscriptionService);
	private filterService = inject(FilterService);
	private endpointService = inject(EndpointsService);

	// Endpoint Related Variables
	endpointForm: FormGroup;

	subscriptionForm: FormGroup;

	// Event Type Selection Variables
	selectedEventTypes: string[] = [];
	eventTypes: EVENT_TYPE[] = [];
	filters: FILTER[] = [];
	subscription!: SUBSCRIPTION;
	filtersMap: Map<string, FILTER> = new Map<string, FILTER>();
	selectedEventType: string = '';
	showFilterDialog = false;
	selectedIndex: number = 0;
	showTransformDialog = false;
	isCreatingSubscription = false;
	createdSubscription = false;

	projectType?: 'incoming' | 'outgoing';
	token: string = this.route.snapshot.queryParams.token;

	// Endpoint Types
	endpointTypes = [
		{ id: 'webhook', name: 'Webhook', icon: 'endpoint-webhook-icon' },
		{ id: 'slack', name: 'Slack', icon: 'endpoint-slack-icon' },
		{ id: 'telegram', name: 'Telegram', icon: 'endpoint-telegram-icon' },
		{ id: 'zapier', name: 'Zapier', icon: 'endpoint-zapier-icon' },
		{ id: 'hubspot', name: 'Hubspot', icon: 'endpoint-hubspot-icon' },
		{ id: 'discord', name: 'Discord', icon: 'endpoint-discord-icon' }
	];
	selectedEndpointType = 'webhook';

	// UI State Variables
	savingEndpoint = false;
	isLoadingEndpointDetails = false;
	endpointCreated = false;
	endpointSecret?: SECRET;
	isLoadingForm = true;
    isTransformFunctionCollapsed = true;

	// Configurations
	configurations = [
		{ uid: 'http_timeout', name: 'Timeout ', show: false, deleted: false },
		{ uid: 'owner_id', name: 'Owner ID ', show: false, deleted: false },
		{ uid: 'rate_limit', name: 'Rate Limit ', show: false, deleted: false },
		{ uid: 'auth', name: 'Auth', show: false, deleted: false },
		{ uid: 'alert_config', name: 'Notifications', show: false, deleted: false },
		{ uid: 'signature', name: 'Signature Format', show: false, deleted: false },
	];

	currentRoute = window.location.pathname.split('/').reverse()[0];

	// Flag to prevent infinite recursion in toggleEventType
	private _isTogglingEventType = false;

	constructor(private route: ActivatedRoute, private cdr: ChangeDetectorRef, public privateService: PrivateService, public licenseService: LicensesService) {
		// Initialize form here in constructor
		this.endpointForm = this.formBuilder.group({
			name: ['', Validators.required],
			url: ['', Validators.compose([Validators.required, Validators.pattern(`^(?:https?|ftp)://[a-zA-Z0-9-]+(?:.[a-zA-Z0-9-]+)+(?::[0-9]+)?/?(?:[a-zA-Z0-9-_.~!$&'()*+,;=:@/?#%]*)?$`)])],
			support_email: ['', Validators.email],
			slack_webhook_url: ['', Validators.pattern(`^(?:https?|ftp)://[a-zA-Z0-9-]+(?:.[a-zA-Z0-9-]+)+(?::[0-9]+)?/?(?:[a-zA-Z0-9-_.~!$&'()*+,;=:@/?#%]*)?$`)],
			secret: [null],
			http_timeout: [null, Validators.pattern('^[-+]?[0-9]+$')],
			description: [null],
			owner_id: [null],
			rate_limit: [null],
			rate_limit_duration: [null],
			authentication: this.formBuilder.group({
				type: ['api_key'],
				api_key: this.formBuilder.group({
					header_name: [''],
					header_value: ['']
				})
			}),
			advanced_signatures: [null]
		});

		this.subscriptionForm = this.formBuilder.group({
			name: ['', Validators.required],
			source_id: [''],
			endpoint_id: [null, Validators.required],
			function: [null],
			eventTypes: this.formBuilder.group({})
		});
	}

	async ngOnInit() {
		this.isLoadingForm = true;

		// Get the endpoint ID from route params if not provided via input
		if (!this.endpointUid) {
			this.endpointUid = this.route.snapshot.params['id'];
		}

		// Set edit mode based on endpoint ID
		if (this.endpointUid && this.endpointUid !== 'new') {
			this.editMode = true;
			this.action = 'update';
		}

		// Load event types for the subscription
		await this.getEventTypes();

		// Make sure events config is shown
		this.toggleConfigForm('events', true);

		// If we're in edit mode, load the endpoint details and related subscription
		if (this.isUpdateAction || this.editMode) {
			await this.getEndpointDetails();
			if (this.endpointUid) {
				await this.getEndpointSubscription();
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

		// Add RBAC check
		if (!(await this.rbacService.userCanAccess('Endpoints|MANAGE'))) {
			this.endpointForm.disable();
		}

		// Force UI update
		this.cdr.detectChanges();
	}

	async getEventTypes() {
		try {
			console.log('Fetching event types...');
			const response = await this.privateService.getEventTypes();
			this.eventTypes = response.data.filter((type: EVENT_TYPE) => !type.deprecated_at);
			console.log('Loaded event types:', this.eventTypes);
			console.log('Event types length:', this.eventTypes.length);
			return;
		} catch (error) {
			console.error('Error loading event types:', error);
			return;
		}
	}

	async getEndpointDetails() {
		this.isLoadingEndpointDetails = true;

		try {
			// Find the endpoint in the list of endpoints
			const response = await this.endpointService.getEndpoint(this.endpointUid);
			const endpointDetails: ENDPOINT = response.data;

			if (!endpointDetails) {
				throw new Error('Endpoint not found');
			}

			this.endpointSecret = endpointDetails?.secrets?.find((it: SECRET) => !it.expires_at);

			// Set the configuration toggles based on endpoint details
			if (endpointDetails.rate_limit_duration) this.toggleConfigForm('rate_limit');
			if (endpointDetails.owner_id) this.toggleConfigForm('owner_id');
			if (endpointDetails.support_email) this.toggleConfigForm('alert_config');
			if (endpointDetails.authentication.api_key.header_value || endpointDetails.authentication.api_key.header_name) this.toggleConfigForm('auth');
			if (endpointDetails.http_timeout) this.toggleConfigForm('http_timeout');

			// Patch the form with endpoint details
			this.endpointForm.patchValue(endpointDetails);

			this.isLoadingEndpointDetails = false;
		} catch (error) {
			console.error('Error loading endpoint details:', error);
			this.generalService.showNotification({
				message: 'Failed to load endpoint details',
				style: 'error'
			});
			this.isLoadingEndpointDetails = false;
		}
	}

	async getEndpointSubscription() {
		try {
			// Check if we have an endpoint ID
			if (!this.endpointUid) {
				console.log('No endpoint UID provided');
				return;
			}

			console.log('Fetching subscriptions for endpoint:', this.endpointUid);
			// Get subscriptions for this endpoint
			const response = await this.privateService.getSubscriptions({ endpointId: this.endpointUid });
			console.log('Subscription response:', response);
			const subscriptions = response.data.content.filter((it: SUBSCRIPTION) => it.endpoint_metadata?.uid === this.endpointUid);

			if (!subscriptions || subscriptions.length === 0) {
				throw new Error('No subscriptions found for this endpoint');
			}

			console.log('Found subscriptions:', subscriptions);

			// If we found a subscription, load it
			this.subscription = subscriptions[0];
			this.subscriptionId = this.subscription.uid;

			// Load event types from the subscription
			if (this.subscription.filter_config?.event_types) {
				this.selectedEventTypes = [...this.subscription.filter_config.event_types];
				console.log('Selected event types from subscription:', this.selectedEventTypes);

				// Create a form group with string keys
				const eventTypesControls: Record<string, any> = {};
				this.selectedEventTypes.forEach((eventType, index) => {
					eventTypesControls[index.toString()] = this.formBuilder.control(eventType);
				});

				// Set the form group values
				this.subscriptionForm.setControl('eventTypes', this.formBuilder.group(eventTypesControls));

				// Show the event types section
				this.toggleConfigForm('events', true);
			}

			// Load filters for this subscription
			await this.loadFiltersForSubscription();
		} catch (error) {
			console.error('Error loading subscription:', error);
		}
	}

	async loadFiltersForSubscription() {
		if (!this.subscriptionId) {
			return;
		}

		try {
			// Get filters for this subscription
			const response = await this.filterService.getFilters(this.subscriptionId);
			this.filters = response.data || [];

			// Clear the map and populate it with the filter data
			this.filtersMap.clear();
			this.filters.forEach((filter: FILTER) => {
				this.filtersMap.set(filter.event_type, { ...filter });
			});

			console.log('Loaded filters:', this.filters);
		} catch (error) {
			console.error('Error loading filters:', error);
		}
	}

	async runEndpointValidation() {
		const configFields: any = {
			http_timeout: ['http_timeout'],
			signature: ['advanced_signatures'],
			rate_limit: ['rate_limit', 'rate_limit_duration'],
			alert_config: ['support_email', 'slack_webhook_url'],
			auth: ['authentication.api_key.header_name', 'authentication.api_key.header_value']
		};

		this.configurations.forEach(config => {
			const fields = configFields[config.uid];
			if (this.showConfig(config.uid)) {
				fields?.forEach((item: string) => {
					this.endpointForm.get(item)?.addValidators(Validators.required);
					this.endpointForm.get(item)?.updateValueAndValidity();
				});
			} else {
				fields?.forEach((item: string) => {
					this.endpointForm.get(item)?.removeValidators(Validators.required);
					this.endpointForm.get(item)?.updateValueAndValidity();
				});
			}
		});
		return;
	}

	selectEndpointType(typeId: string) {
		this.selectedEndpointType = typeId;
	}

	toggleConfigForm(configValue: string, deleted?: boolean) {
		this.configurations.forEach(config => {
			if (config.uid === configValue) {
				config.show = !config.show;
				config.deleted = deleted ?? false;
			}
		});
	}

	setConfigFormDeleted(configValue: string, deleted: boolean) {
		this.configurations.forEach(config => {
			if (config.uid === configValue) {
				config.deleted = deleted;
			}
		});
	}

	showConfig(configValue: string): boolean {
		return this.configurations.find(config => config.uid === configValue)?.show || false;
	}

	configDeleted(configValue: string): boolean {
		return this.configurations.find(config => config.uid === configValue)?.deleted || false;
	}

    toggleEventTypeSelection(eventType: string) {
        const isWildcard = eventType === '*';
        const index = this.selectedEventTypes.indexOf(eventType);

        if (index > -1) {
            // If already selected, remove it
            this.selectedEventTypes.splice(index, 1);
            return;
        }

        if (isWildcard) {
            // Selecting wildcard (*) - clear all event types first
            this.selectedEventTypes = ['*'];
        } else {
            // If a specific event type is selected, remove wildcard (*) if it's present
            const wildcardIndex = this.selectedEventTypes.indexOf('*');
            if (wildcardIndex > -1) {
                this.selectedEventTypes.splice(wildcardIndex, 1);
            }
            this.selectedEventTypes.push(eventType);
        }
    }


    openFilterDialog(index: number) {
        const eventType = this.eventTypes[index].name;
        this.selectedEventType = eventType || '';
        this.selectedIndex = index;

        // Ensure a filter entry exists for this event type
        if (!this.filtersMap.has(eventType)) {
            this.filtersMap.set(eventType, {
                uid: '', // Assigned by backend
                subscription_id: '',
                event_type: eventType,
                headers: {},
                body: {},
                is_new: true
            });

            this._syncFiltersArrayWithMap();
        }

        this.showFilterDialog = true;
    }

    onSaveFilter(schema: any) {
		if (!this.selectedEventType) {
			console.error('No event type selected for filter');
			return;
		}

		// Get the existing filter from the map or create a default object
		const existingFilter = this.filtersMap.get(this.selectedEventType) || {
			uid: '', // Will be assigned by backend
			subscription_id: '',
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

		// Close the filter dialog
		this.showFilterDialog = false;
	}

	// Save endpoint and create subscription
	async saveEndpointAndSubscription() {
		// First validate the endpoint form
		await this.runEndpointValidation();

		if (this.endpointForm.invalid) {
			this.savingEndpoint = false;
			this.endpointForm.markAllAsTouched();
			this.generalService.showNotification({ message: 'Please fill all required fields', style: 'error' });
			return;
		}

		// Handle rate limit deletion
		let rateLimitDeleted = !this.showConfig('rate_limit') && this.configDeleted('rate_limit');
		if (rateLimitDeleted) {
			const configKeys = ['rate_limit', 'rate_limit_duration'];
			configKeys.forEach(key => {
				this.endpointForm.value[key] = 0;
				this.endpointForm.get(`${key}`)?.patchValue(0);
			});
			this.setConfigFormDeleted('rate_limit', false);
		}

		this.savingEndpoint = true;
		const endpointValue = structuredClone(this.endpointForm.value);

		// Remove authentication if not provided
		if (!this.endpointForm.value.authentication.api_key.header_name && !this.endpointForm.value.authentication.api_key.header_value) {
			delete endpointValue.authentication;
		}

		try {
			// STEP 1: Create or update the endpoint
			const response =
				this.isUpdateAction || this.editMode
					? await this.createEndpointService.editEndpoint({
							endpointId: this.endpointUid || '',
							body: endpointValue
					  })
					: await this.createEndpointService.addNewEndpoint({
							body: endpointValue
					  });

			const createdEndpoint = response.data;
			this.endpointSecret = createdEndpoint?.secrets?.find((it: SECRET) => !it.expires_at);
			this.endpointCreated = true;
			this.endpointUid = createdEndpoint.uid;

			// Show success notification for endpoint creation
			this.generalService.showNotification({
				message: this.isUpdateAction || this.editMode ? 'Endpoint updated successfully' : 'Endpoint created successfully',
				style: 'success'
			});

			// STEP 2: Create a subscription with event types
			if (this.subscriptionId == 'new' || !(this.isUpdateAction || this.editMode)) {
				// Generate a subscription name based on endpoint name
				const uuid = this.generateUUID().substring(0, 8);
				const subscriptionName = `${createdEndpoint.name}-${uuid}`;

				// Prepare subscription data
				const subscriptionData = {
					name: subscriptionName,
					endpoint_id: createdEndpoint.uid,
					filter_config: {
						event_types: this.selectedEventTypes,
						filter: {
							headers: {},
							body: {}
						}
					}
				};

				try {
					// Create the subscription
					const subscriptionResponse = await this.createSubscriptionService.createSubscription(subscriptionData);
					this.subscription = subscriptionResponse.data;
					this.subscriptionId = this.subscription.uid;

					// STEP 3: Create filters for the subscription
					if (this.filtersMap.size > 0) {
						try {
							// Prepare filters to create
							const filtersToCreate = Array.from(this.filtersMap.values()).map(filter => ({
								subscription_id: this.subscriptionId,
								event_type: filter.event_type,
								headers: filter.headers || {},
								body: filter.body || {}
							}));

							// Create filters if we have any
							if (filtersToCreate.length > 0) {
								await this.filterService.createFilters(this.subscriptionId, filtersToCreate);
							}

							this.generalService.showNotification({
								message: 'Subscription created with filters',
								style: 'success'
							});
						} catch (error) {
							console.error('Error creating filters:', error);
							this.generalService.showNotification({
								message: 'Subscription created but filters could not be added',
								style: 'warning'
							});
						}
					} else {
						this.generalService.showNotification({
							message: 'Subscription created successfully',
							style: 'success'
						});
					}

					this.createdSubscription = true;
				} catch (error) {
					console.error('Error creating subscription:', error);
					this.generalService.showNotification({
						message: 'Endpoint created but subscription could not be created',
						style: 'warning'
					});
				}
			} else {
				// We're in update mode and have a valid subscription ID
				try {
					// Prepare subscription data for update
					const subscriptionData = {
						filter_config: {
							event_types: this.selectedEventTypes,
							filter: {
								headers: {},
								body: {}
							}
						}
					};

					// Update the subscription
					const subscriptionResponse = await this.createSubscriptionService.updateSubscription({
						data: subscriptionData,
						id: this.subscriptionId
					});

					this.subscription = subscriptionResponse.data;

					// Handle filters update for existing subscription
					await this.updateFiltersForSubscription();

					this.generalService.showNotification({
						message: 'Subscription updated successfully',
						style: 'success'
					});

					this.createdSubscription = true;
				} catch (error) {
					console.error('Error updating subscription:', error);
					this.generalService.showNotification({
						message: 'Endpoint updated but subscription update failed',
						style: 'warning'
					});
				}
			}

			// Final notification of success
			this.onAction.emit({
				action: this.endpointUid && this.editMode ? 'update' : 'save',
				data: createdEndpoint
			});

			this.savingEndpoint = false;
			return response;
		} catch (error) {
			console.error('Error creating endpoint:', error);
			this.generalService.showNotification({
				message: 'Failed to create endpoint',
				style: 'error'
			});
			this.endpointCreated = false;
			this.savingEndpoint = false;
			return;
		}
	}

	// Helper method to generate a UUID v4
	private generateUUID(): string {
		return 'xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx'.replace(/[xy]/g, function (c) {
			const r = (Math.random() * 16) | 0,
				v = c == 'x' ? r : (r & 0x3) | 0x8;
			return v.toString(16);
		});
	}

	get shouldShowBorder(): number {
		return this.configurations.filter(config => config.show).length;
	}

	get isUpdateAction(): boolean {
		return this.endpointUid && this.endpointUid !== 'new' && this.currentRoute !== 'setup';
	}

	getFunction(subscriptionFunction: any) {
		if (subscriptionFunction) this.subscriptionForm.get('function')?.patchValue(subscriptionFunction);
		this.showTransformDialog = false;
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

	get eventTypesFormGroup(): FormGroup {
		return this.subscriptionForm.get('eventTypes') as FormGroup;
	}

	// Helper method to update filters for an existing subscription
	private async updateFiltersForSubscription(): Promise<void> {
		if (!this.subscriptionId) {
			console.error('Cannot update filters: No subscription ID');
			return;
		}

		try {
			// Get existing filters
			const existingFiltersResponse = await this.filterService.getFilters(this.subscriptionId);
			const existingFiltersContent = existingFiltersResponse.data || [];

			// Create a map of existing filters by event type for easy lookup
			const existingFiltersByEventType: { [key: string]: any } = {};
			existingFiltersContent.forEach((filter: any) => {
				existingFiltersByEventType[filter.event_type] = filter;
			});

			// Identify filters to update and create
			const filtersToUpdate: any[] = [];
			const filtersToCreate: any[] = [];

			// Process each filter in the map
			this.filtersMap.forEach((filter, eventType) => {
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
						body: filter.body || {}
					});
				}
			});

			// Process deletes: Look for filters that exist in existingFiltersByEventType
			// but not in filtersMap (they've been removed and need to be deleted)
			const filtersToDelete: string[] = [];

			Object.keys(existingFiltersByEventType).forEach(eventType => {
				if (!this.filtersMap.has(eventType)) {
					// This filter exists on the server but is no longer in our local map
					filtersToDelete.push(existingFiltersByEventType[eventType].uid);
				}
			});

			// Execute the API operations
			const operations = [];

			// Create new filters
			if (filtersToCreate.length > 0) {
				operations.push(this.filterService.createFilters(this.subscriptionId, filtersToCreate));
			}

			// Update existing filters
			if (filtersToUpdate.length > 0) {
				operations.push(this.filterService.bulkUpdateFilters(this.subscriptionId, filtersToUpdate));
			}

			// Delete filters that were removed
			if (filtersToDelete.length > 0) {
				// Note: You need to implement a method for deleting filters
				// operations.push(this.filterService.deleteFilters(this.subscriptionId, filtersToDelete));
				console.log('Filters to delete:', filtersToDelete);
				// For now we'll just log them
			}

			// Wait for all operations to complete
			if (operations.length > 0) {
				await Promise.all(operations);
			}

			console.log('Filters updated successfully');
			return;
		} catch (error) {
			console.error('Error updating filters:', error);
			throw error;
		}
	}

	// Check if an event type is selected
	isEventTypeSelected(eventTypeName: string): boolean {
		return this.selectedEventTypes.includes(eventTypeName);
	}

	// Toggle an event type on/off with special handling for wildcard (*)
	toggleEventType(eventTypeName: string, isAutoSelect = false): void {
		// Prevent toggling if the subscription is being created/updated
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

	// Helper method to sync filters array with filtersMap
	private _syncFiltersArrayWithMap(): void {
		// Convert the map values to an array and assign to filters
		this.filters = Array.from(this.filtersMap.values());
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

	toEventTypesString(){
		return this.eventTypes.map(e => e.name).filter(e=> e !== '*')
	}

    toggleTransformFunction() {
        this.isTransformFunctionCollapsed = !this.isTransformFunctionCollapsed;
    }

}
