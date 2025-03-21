import { Component, EventEmitter, Input, OnInit, Output, inject, ViewEncapsulation } from '@angular/core';
import { CommonModule, NgOptimizedImage } from '@angular/common';
import { ControlContainer, FormArray, FormBuilder, FormControl, FormGroup, FormGroupDirective, ReactiveFormsModule, Validators } from '@angular/forms';
import { ActivatedRoute, Router } from '@angular/router';

import { GeneralService } from '../../services/general/general.service';
import { FilterService } from '../../private/components/create-subscription/filter.service';
import { RbacService } from '../../services/rbac/rbac.service';
import { LicensesService } from '../../services/licenses/licenses.service';
import { PrivateService } from '../../private/private.service';
import { CreateEndpointService } from '../../private/components/create-endpoint/create-endpoint.service';
import { CreateSubscriptionService } from '../../private/components/create-subscription/create-subscription.service';

import { InputDirective, InputErrorComponent, InputFieldDirective, LabelComponent } from '../../components/input/input.component';
import { ButtonComponent } from '../../components/button/button.component';
import { RadioComponent } from '../../components/radio/radio.component';
import { TooltipComponent } from '../../components/tooltip/tooltip.component';
import { CardComponent } from '../../components/card/card.component';
import { ToggleComponent } from '../../components/toggle/toggle.component';
import { FormLoaderComponent } from '../../components/form-loader/form-loader.component';
import { PermissionDirective } from '../../private/components/permission/permission.directive';
import { CreateTransformFunctionComponent } from '../../private/components/create-transform-function/create-transform-function.component';
import { CreateSubscriptionFilterComponent } from '../../private/components/create-subscription-filter/create-subscription-filter.component';
import { CreatePortalTransformFunctionComponent } from '../create-portal-transform-function/create-portal-transform-function.component';

import { ENDPOINT, SECRET } from '../../models/endpoint.model';
import { EVENT_TYPE } from '../../models/event.model';
import { FILTER } from '../../models/filter.model';
import { SUBSCRIPTION } from '../../models/subscription';
import {EndpointsService} from "../../private/pages/project/endpoints/endpoints.service";
import {NotificationComponent} from "../../components/notification/notification.component";
import {ConfigButtonComponent} from "../../private/components/config-button/config-button.component";

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
		ToggleComponent,
		PermissionDirective,
		CreateTransformFunctionComponent,
		CreateSubscriptionFilterComponent,
		CreatePortalTransformFunctionComponent,
		NotificationComponent,
		ConfigButtonComponent
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

	// Configurations
	configurations = [
		{ uid: 'http_timeout', name: 'Timeout ', show: false, deleted: false },
		{ uid: 'owner_id', name: 'Owner ID ', show: false, deleted: false },
		{ uid: 'rate_limit', name: 'Rate Limit ', show: false, deleted: false },
		{ uid: 'auth', name: 'Auth', show: false, deleted: false },
		{ uid: 'alert_config', name: 'Notifications', show: false, deleted: false },
		{ uid: 'signature', name: 'Signature Format', show: false, deleted: false },
		{ uid: 'events', name: 'Event Types', show: true, deleted: false }
	];

	currentRoute = window.location.pathname.split('/').reverse()[0];

	constructor(private route: ActivatedRoute, public privateService: PrivateService, private router: Router, public licenseService: LicensesService) {
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
		// Load event types for the subscription
		await this.getEventTypes();

		// If we're in edit mode, load the endpoint details and related subscription
		if (this.isUpdateAction || this.editMode) {
			await this.getEndpointDetails();
			if (this.endpointUid) {
				await this.getEndpointSubscription();
			}
		} else {
			// Initialize selectedEventTypes with wildcard if available, otherwise first event type
			this.initializeDefaultEventType();
		}

		// Add RBAC check
		if (!(await this.rbacService.userCanAccess('Endpoints|MANAGE'))) {
			this.endpointForm.disable();
		}
	}

	initializeDefaultEventType() {
		if (this.eventTypes.length > 0) {
			// Prefer wildcard if available
			const wildcardExists = this.eventTypes.some(type => type.name === '*');
			if (wildcardExists) {
				this.selectedEventTypes = ['*'];
			} else {
				this.selectedEventTypes = [this.eventTypes[0].name];
			}
		}
	}

	async getEventTypes() {
		try {
			const response = await this.privateService.getEventTypes();
			this.eventTypes = response.data.filter((type: EVENT_TYPE) => !type.deprecated_at);
			console.log("event types:", this.eventTypes);
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
				return;
			}

			// Get subscriptions for this endpoint
			const response = await this.privateService.getSubscriptions({ endpointId: this.endpointUid });
			const subscriptions = response.data.content;


			if (!subscriptions && subscriptions.length == 0) {
				throw new Error('Endpoint not found');
			}

			console.log("sub event types:", subscriptions[0].filter_config.event_types);

			// If we found a subscription, load it
			this.subscription = subscriptions[0];
			this.subscriptionId = this.subscription.uid;

			// Load event types from the subscription
			if (this.subscription.filter_config?.event_types) {
				this.selectedEventTypes = [...this.subscription.filter_config.event_types];

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

	// Event Type Selection Methods

	toggleEventType(eventTypeName: string): void {
		const index = this.selectedEventTypes.indexOf(eventTypeName);
		const isWildcard = eventTypeName === '*';

		if (index !== -1) {
			// If already selected, remove it (unless it's the only one left)
			if (this.selectedEventTypes.length > 1) {
				this.selectedEventTypes.splice(index, 1);
				this.filtersMap.delete(eventTypeName);
			}
			return;
		}

		// If selecting the wildcard (*) event type, remove all other event types
		if (isWildcard) {
			this.selectedEventTypes = [];
			this.filtersMap.clear();
		}
		// If selecting a specific event type and wildcard is already selected, remove the wildcard
		else if (this.selectedEventTypes.includes('*')) {
			const wildcardIndex = this.selectedEventTypes.indexOf('*');
			this.selectedEventTypes.splice(wildcardIndex, 1);
			this.filtersMap.delete('*');
		}

		// Add the newly selected event type
		this.selectedEventTypes.push(eventTypeName);

		// Add a filter for this event type to the map
		this.filtersMap.set(eventTypeName, {
			uid: '', // Will be assigned by backend
			subscription_id: '',
			event_type: eventTypeName,
			headers: {},
			is_new: true,
			body: {}
		});

		// Sync with filters array for compatibility
		this._syncFiltersArrayWithMap();
	}

	isEventTypeSelected(eventTypeName: string): boolean {
		return this.selectedEventTypes.includes(eventTypeName);
	}

	openFilterDialog(eventType: string) {
		this.selectedEventType = eventType || '';

		// For backward compatibility
		this.selectedIndex = this.filters.findIndex(item => item.event_type === eventType);

		// Ensure we have a filter entry for this event type in the map
		if (!this.filtersMap.has(eventType) && eventType) {
			// Create a new filter entry if it doesn't exist
			this.filtersMap.set(eventType, {
				uid: '', // Will be assigned by backend
				subscription_id: '',
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

	// Helper method to sync filters array with filtersMap
	private _syncFiltersArrayWithMap(): void {
		// Convert the map values to an array and assign to filters
		this.filters = Array.from(this.filtersMap.values());
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

			// STEP 2: Create or update subscription with event types
			if (!this.subscriptionId || !(this.isUpdateAction || this.editMode)) {
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
				// We're in update mode and have a subscription ID
				try {
					// Prepare subscription data for update
					const subscriptionData = {
						endpoint_id: createdEndpoint.uid,
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
			this.subscriptionForm.markAllAsTouched();
			return;
		}

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

			this.onAction.emit({ data: this.subscription, action: this.action == 'update' ? 'update' : 'create' });
		} catch (error) {
			this.createdSubscription = false;
			this.isCreatingSubscription = false;
		}
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

	/** Copy text to clipboard */
	copyToClipboard(text: string): void {
		navigator.clipboard
			.writeText(text)
			.then(() => {
				console.log('Text copied to clipboard');
				// Show a notification if needed
			})
			.catch(err => {
				console.error('Could not copy text: ', err);
			});
	}
}
