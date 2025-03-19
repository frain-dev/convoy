import { Component, EventEmitter, Input, OnInit, Output, inject } from '@angular/core';
import { CommonModule } from '@angular/common';
import { FormBuilder, FormGroup, ReactiveFormsModule, Validators } from '@angular/forms';
import { InputDirective, InputErrorComponent, InputFieldDirective, LabelComponent } from 'src/app/components/input/input.component';
import { ButtonComponent } from 'src/app/components/button/button.component';
import { RadioComponent } from 'src/app/components/radio/radio.component';
import { TooltipComponent } from 'src/app/components/tooltip/tooltip.component';
import { CardComponent } from 'src/app/components/card/card.component';
import { ToggleComponent } from 'src/app/components/toggle/toggle.component';
import { FormLoaderComponent } from 'src/app/components/form-loader/form-loader.component';
import { PermissionDirective } from '../permission/permission.directive';
import { CopyButtonComponent } from 'src/app/components/copy-button/copy-button.component';
import { TagComponent } from 'src/app/components/tag/tag.component';
import { NotificationComponent } from 'src/app/components/notification/notification.component';
import { ConfigButtonComponent } from '../config-button/config-button.component';
import { ActivatedRoute, Router } from '@angular/router';
import { GeneralService } from 'src/app/services/general/general.service';
import { RbacService } from 'src/app/services/rbac/rbac.service';
import { LicensesService } from 'src/app/services/licenses/licenses.service';
import { PrivateService } from '../../private.service';
import { CreateEndpointService } from '../create-endpoint/create-endpoint.service';
import { CreateSubscriptionService } from '../create-subscription/create-subscription.service';
import { FilterService } from '../create-subscription/filter.service';
import { ENDPOINT, SECRET } from 'src/app/models/endpoint.model';
import { SUBSCRIPTION_CONFIG } from 'src/app/models/subscription';
import { EVENT_TYPE } from 'src/app/models/event.model';
import { FILTER } from 'src/app/models/filter.model';
import { DialogDirective, DialogHeaderComponent } from "../../../components/dialog/dialog.directive";

@Component({
	selector: 'convoy-create-endpoint-subscription',
	standalone: true,
	imports: [
		CommonModule,
		ReactiveFormsModule,
		InputDirective,
		InputErrorComponent,
		InputFieldDirective,
		LabelComponent,
		ButtonComponent,
		RadioComponent,
		TooltipComponent,
		CardComponent,
		ToggleComponent,
		FormLoaderComponent,
		PermissionDirective,
		NotificationComponent,
		ConfigButtonComponent,
		CopyButtonComponent,
		TagComponent,
		DialogDirective,
		DialogHeaderComponent
	],
	templateUrl: './create-endpoint-subscription.component.html',
	styleUrls: ['./create-endpoint-subscription.component.scss']
})
export class CreateEndpointSubscriptionComponent implements OnInit {
	@Input('editMode') editMode = false;
	@Input('showAction') showAction: 'true' | 'false' = 'false';
	@Input('type') type: 'in-app' | 'portal' | 'subscription' = 'in-app';
	@Input('endpointId') endpointUid = this.route.snapshot.params.id;
	@Output() onAction = new EventEmitter<any>();

	// Injected Services
	private rbacService = inject(RbacService);
	private formBuilder = inject(FormBuilder);
	private generalService = inject(GeneralService);
	private createEndpointService = inject(CreateEndpointService);
	private createSubscriptionService = inject(CreateSubscriptionService);
	private filterService = inject(FilterService);

	// Endpoint Related Variables
	endpointForm: FormGroup;

	// Event Type Selection Variables
	selectedEventTypes: string[] = [];
	eventTypes: EVENT_TYPE[] = [];
	filters: FILTER[] = [];
	filtersMap: Map<string, FILTER> = new Map<string, FILTER>();
	selectedEventType: string = '';
	showFilterDialog = false;
	selectedIndex: number = 0;

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
	endpointConfigurations = [
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
	}

	async ngOnInit() {
		// Load event types for the subscription
		await this.getEventTypes();

		// If we're in edit mode, load the endpoint details
		if (this.isUpdateAction || this.editMode) {
			this.getEndpointDetails();
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
			const response = await this.privateService.getEndpoints({ q: this.endpointUid });
			const endpoints = response.data.content;
			const endpointDetails: ENDPOINT = endpoints.find((endpoint: ENDPOINT) => endpoint.uid === this.endpointUid);

			if (!endpointDetails) {
				throw new Error('Endpoint not found');
			}

			this.endpointSecret = endpointDetails?.secrets?.find(secret => !secret.expires_at);

			// Set the configuration toggles based on endpoint details
			if (endpointDetails.rate_limit_duration) this.toggleConfigForm('rate_limit');
			if (endpointDetails.owner_id) this.toggleConfigForm('owner_id');
			if (endpointDetails.support_email) this.toggleConfigForm('alert_config');
			if (endpointDetails.authentication.api_key.header_value || endpointDetails.authentication.api_key.header_name) this.toggleConfigForm('auth');
			if (endpointDetails.http_timeout) this.toggleConfigForm('http_timeout');

			// Patch the form with endpoint details
			this.endpointForm.patchValue(endpointDetails);

			this.isLoadingEndpointDetails = false;
		} catch {
			this.isLoadingEndpointDetails = false;
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

		this.endpointConfigurations.forEach(config => {
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
		this.endpointConfigurations.forEach(config => {
			if (config.uid === configValue) {
				config.show = !config.show;
				config.deleted = deleted ?? false;
			}
		});
	}

	setConfigFormDeleted(configValue: string, deleted: boolean) {
		this.endpointConfigurations.forEach(config => {
			if (config.uid === configValue) {
				config.deleted = deleted;
			}
		});
	}

	showConfig(configValue: string): boolean {
		return this.endpointConfigurations.find(config => config.uid === configValue)?.show || false;
	}

	configDeleted(configValue: string): boolean {
		return this.endpointConfigurations.find(config => config.uid === configValue)?.deleted || false;
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
			return this.endpointForm.markAllAsTouched();
		}

		let rateLimitDeleted = !this.showConfig('rate_limit') && this.configDeleted('rate_limit');
		if (rateLimitDeleted) {
			const configKeys = ['rate_limit', 'rate_limit_duration'];
			configKeys.forEach(key => {
				this.endpointForm.value[key] = 0; // element type = number
				this.endpointForm.get(`${key}`)?.patchValue(0);
			});
			this.setConfigFormDeleted('rate_limit', false);
		}

		this.savingEndpoint = true;
		const endpointValue = structuredClone(this.endpointForm.value);

		if (!this.endpointForm.value.authentication.api_key.header_name && !this.endpointForm.value.authentication.api_key.header_value) {
			delete endpointValue.authentication;
		}

		try {
			// Step 1: Create or update the endpoint
			const response = this.isUpdateAction || this.editMode ? await this.createEndpointService.editEndpoint({ endpointId: this.endpointUid || '', body: endpointValue }) : await this.createEndpointService.addNewEndpoint({ body: endpointValue });

			const createdEndpoint = response.data;
			this.endpointCreated = true;

			// Step 2: If creating a new endpoint, automatically create a subscription with event types
			if (!this.isUpdateAction && !this.editMode && createdEndpoint) {
				await this.createSubscriptionForEndpoint(createdEndpoint);
			}

			this.generalService.showNotification({ message: response.message, style: 'success' });
			this.onAction.emit({
				action: this.endpointUid && this.editMode ? 'update' : 'save',
				data: createdEndpoint
			});

			this.endpointForm.reset();
			return response;
		} catch {
			this.endpointCreated = false;
			this.savingEndpoint = false;
			return;
		}
	}

	private async createSubscriptionForEndpoint(endpoint: ENDPOINT) {
		// Generate a UUID v4
		const uuid = this.generateUUID();
		const subscriptionName = `${endpoint.name}-${uuid}`;

		const subscriptionData = {
			name: subscriptionName,
			endpoint_id: endpoint.uid,
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
			const subscription = subscriptionResponse.data;

			// If we have filters, save them
			if (this.filtersMap.size > 0) {
				try {
					// Convert filters from map to array
					const filtersToCreate = Array.from(this.filtersMap.values()).map(filter => ({
						subscription_id: subscription.uid,
						event_type: filter.event_type,
						headers: filter.headers || {},
						body: filter.body || {}
					}));

					// Create filters
					if (filtersToCreate.length > 0) {
						await this.filterService.createFilters(subscription.uid, filtersToCreate);
					}
				} catch (error) {
					console.error('Error saving filters:', error);
				}
			}

			return subscription;
		} catch (error) {
			console.error('Error creating subscription:', error);
			throw error;
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
		return this.endpointConfigurations.filter(config => config.show).length;
	}

	get isUpdateAction(): boolean {
		return this.endpointUid && this.endpointUid !== 'new' && this.currentRoute !== 'setup';
	}
}
