import {Component, EventEmitter, inject, Input, OnInit, Output} from '@angular/core';
import {CommonModule} from '@angular/common';
import {AbstractControl, FormBuilder, FormGroup, ReactiveFormsModule, ValidatorFn, Validators} from '@angular/forms';
import {
    InputDirective,
    InputErrorComponent,
    InputFieldDirective,
    LabelComponent
} from 'src/app/components/input/input.component';
import {ButtonComponent} from 'src/app/components/button/button.component';
import {RadioComponent} from 'src/app/components/radio/radio.component';
import {TooltipComponent} from 'src/app/components/tooltip/tooltip.component';
import {GeneralService} from 'src/app/services/general/general.service';
import {ActivatedRoute, Router} from '@angular/router';
import {CardComponent} from 'src/app/components/card/card.component';
import {CreateEndpointService} from './create-endpoint.service';
import {PrivateService} from '../../private.service';
import {FormLoaderComponent} from 'src/app/components/form-loader/form-loader.component';
import {PermissionDirective} from '../permission/permission.directive';
import {RbacService} from 'src/app/services/rbac/rbac.service';
import {ENDPOINT, SECRET} from 'src/app/models/endpoint.model';
import {EndpointsService} from '../../pages/project/endpoints/endpoints.service';
import {NotificationComponent} from 'src/app/components/notification/notification.component';
import {ConfigButtonComponent} from '../config-button/config-button.component';
import {CopyButtonComponent} from 'src/app/components/copy-button/copy-button.component';
import {LicensesService} from 'src/app/services/licenses/licenses.service';
import {TagComponent} from 'src/app/components/tag/tag.component';
import {SelectComponent} from 'src/app/components/select/select.component';
import {SettingsService} from '../../pages/settings/settings.service';

// Custom validators that skip validation for [REDACTED] placeholder
function mtlsCertValidator(): ValidatorFn {
	return (control: AbstractControl): {[key: string]: any} | null => {
		const value = control.value;
		// Skip validation if empty or [REDACTED] (unchanged from server)
		if (!value || value === '[REDACTED]') {
			return null;
		}
		// Validate PEM format
		const certPattern = /^-----BEGIN CERTIFICATE-----[\s\S]*-----END CERTIFICATE-----\s*$/;
		return certPattern.test(value) ? null : { pattern: { value } };
	};
}

function mtlsKeyValidator(): ValidatorFn {
	return (control: AbstractControl): {[key: string]: any} | null => {
		const value = control.value;
		// Skip validation if empty or [REDACTED] (unchanged from server)
		if (!value || value === '[REDACTED]') {
			return null;
		}
		// Validate PEM format for private key
		const keyPattern = /^-----BEGIN (RSA )?PRIVATE KEY-----[\s\S]*-----END (RSA )?PRIVATE KEY-----\s*$/;
		return keyPattern.test(value) ? null : { pattern: { value } };
	};
}

@Component({
	selector: 'convoy-create-endpoint',
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
        FormLoaderComponent,
        PermissionDirective,
        NotificationComponent,
        ConfigButtonComponent,
        CopyButtonComponent,
        TagComponent,
        SelectComponent
    ],
	templateUrl: './create-endpoint.component.html',
	styleUrls: ['./create-endpoint.component.scss']
})
export class CreateEndpointComponent implements OnInit {
	@Input('editMode') editMode = false;
	@Input('showAction') showAction: 'true' | 'false' = 'false';
	@Input('type') type: 'in-app' | 'portal' | 'subscription' = 'in-app';
	@Output() onAction = new EventEmitter<any>();
	savingEndpoint = false;
	isLoadingEndpointDetails = false;
	isLoadingEndpoints = false;
	testingOAuth2 = false;
	oauth2TestResult: any = null;
	selectedAuthType: string = 'api_key';
	selectedOAuth2AuthType: string = 'shared_secret';
	selectedKeyType: string = 'EC';
	addNewEndpointForm: FormGroup = this.formBuilder.group({
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
			}),
			oauth2: this.formBuilder.group({
				url: [''],
				client_id: [''],
				authentication_type: ['shared_secret'],
				client_secret: [''],
				grant_type: ['client_credentials'],
				scope: [''],
				signing_key: this.formBuilder.group({
					kty: ['EC'],
					crv: ['P-256'],
					kid: [''],
					x: [''],
					y: [''],
					d: [''],
					// RSA fields
					n: [''],
					e: [''],
					p: [''],
					q: [''],
					dp: [''],
					dq: [''],
					qi: ['']
				}),
				signing_algorithm: ['ES256'],
				issuer: [''],
				subject: [''],
				field_mapping: this.formBuilder.group({
					access_token: ['access_token'],
					token_type: ['token_type'],
					expires_in: ['expires_in']
				}),
				expiry_time_unit: ['seconds']
			})
		}),
		advanced_signatures: [null],
		content_type: ['application/json'],
		mtls_client_cert: this.formBuilder.group({
			client_cert: ['', [mtlsCertValidator()]],
			client_key: ['', [mtlsKeyValidator()]]
		})
	});
	token: string = this.route.snapshot.params.token;
	@Input('endpointId') endpointUid = this.route.snapshot.params.id;
	enableMoreConfig = false;
	configurations = [
		{ uid: 'content_type', name: 'Content Type', show: false, deleted: false },
		{ uid: 'http_timeout', name: 'Timeout ', show: false, deleted: false }
	];
	contentTypeOptions = [
		{ uid: 'application/json', name: 'JSON (application/json)' },
		{ uid: 'application/x-www-form-urlencoded', name: 'Form Data (application/x-www-form-urlencoded)' }
	];
	selectedContentType = 'application/json';
	endpointCreated: boolean = false;
	endpointSecret?: SECRET;
	currentRoute = window.location.pathname.split('/').reverse()[0];
	mtlsFeatureEnabled = false;
	oauth2FeatureEnabled = false;
	organisationId!: string;
	private rbacService = inject(RbacService);

	constructor(
		private formBuilder: FormBuilder,
		private generalService: GeneralService,
		private createEndpointService: CreateEndpointService,
		private route: ActivatedRoute,
		public privateService: PrivateService,
		private router: Router,
		private endpointService: EndpointsService,
		public licenseService: LicensesService,
		private settingsService: SettingsService
	) {}

	async ngOnInit() {
		this.getOrganisationId();
		await this.checkMTLSFeatureFlag();
		await this.checkOAuth2FeatureFlag();

		if (this.type !== 'portal')
			this.configurations.push(
				{ uid: 'owner_id', name: 'Owner ID ', show: false, deleted: false },
				{ uid: 'rate_limit', name: 'Rate Limit ', show: false, deleted: false },
				{ uid: 'auth', name: 'Auth', show: false, deleted: false },
				{ uid: 'alert_config', name: 'Notifications', show: false, deleted: false },
				{ uid: 'signature', name: 'Signature Format', show: false, deleted: false },
				{ uid: 'mtls', name: 'mTLS Client Certificate', show: false, deleted: false },
			);

		// Initialize selectedAuthType from form
		this.selectedAuthType = this.addNewEndpointForm.get('authentication.type')?.value || 'api_key';

		// Watch for authentication type changes
		this.addNewEndpointForm.get('authentication.type')?.valueChanges.subscribe(authType => {
			this.selectedAuthType = authType || 'api_key';
			this.updateOAuth2Validators(authType);
		});
		
		// Initialize OAuth2 validators based on current auth type
		this.updateOAuth2Validators(this.selectedAuthType);

		// Initialize selectedOAuth2AuthType from form
		this.selectedOAuth2AuthType = this.addNewEndpointForm.get('authentication.oauth2.authentication_type')?.value || 'shared_secret';

		// Initialize selectedKeyType from form
		this.selectedKeyType = this.addNewEndpointForm.get('authentication.oauth2.signing_key.kty')?.value || 'EC';

		// Watch for OAuth2 authentication type changes to update validators
		this.addNewEndpointForm.get('authentication.oauth2.authentication_type')?.valueChanges.subscribe(authType => {
			this.selectedOAuth2AuthType = authType || 'shared_secret';
			const clientSecretControl = this.addNewEndpointForm.get('authentication.oauth2.client_secret');
			const issuerControl = this.addNewEndpointForm.get('authentication.oauth2.issuer');
			const subjectControl = this.addNewEndpointForm.get('authentication.oauth2.subject');

			if (authType === 'shared_secret') {
				// Require client_secret, remove validators from assertion fields
				clientSecretControl?.setValidators([Validators.required]);
				clientSecretControl?.updateValueAndValidity();
				issuerControl?.clearValidators();
				issuerControl?.updateValueAndValidity();
				subjectControl?.clearValidators();
				subjectControl?.updateValueAndValidity();
				// Clear all JWK field validators
				this.updateJWKValidators('shared_secret', null);
			} else if (authType === 'client_assertion') {
				// Require assertion fields, remove validator from client_secret
				clientSecretControl?.clearValidators();
				clientSecretControl?.updateValueAndValidity();
				issuerControl?.setValidators([Validators.required]);
				issuerControl?.updateValueAndValidity();
				subjectControl?.setValidators([Validators.required]);
				subjectControl?.updateValueAndValidity();
				// Update JWK validators based on current key type
				const currentKeyType = this.selectedKeyType || this.addNewEndpointForm.get('authentication.oauth2.signing_key.kty')?.value || 'EC';
				this.updateJWKValidators('client_assertion', currentKeyType);
			}
		});

		if (!this.endpointUid) this.endpointUid = this.route.snapshot.params.id;
		if ((this.isUpdateAction || this.editMode) && this.type !== 'subscription') this.getEndpointDetails();
		if (!(await this.rbacService.userCanAccess('Endpoints|MANAGE'))) this.addNewEndpointForm.disable();
	}

	onAuthTypeChange(value: string) {
		// Update the form control value
		this.addNewEndpointForm.get('authentication.type')?.setValue(value, { emitEvent: true });
		this.selectedAuthType = value || 'api_key';
		this.updateOAuth2Validators(value);
	}

	onOAuth2AuthTypeChange(value: string) {
		// Update the form control value
		this.addNewEndpointForm.get('authentication.oauth2.authentication_type')?.setValue(value, { emitEvent: true });
		this.selectedOAuth2AuthType = value || 'shared_secret';
	}

	onKeyTypeChange(value: string) {
		this.addNewEndpointForm.get('authentication.oauth2.signing_key.kty')?.setValue(value, { emitEvent: true });
		this.selectedKeyType = value || 'EC';
		// Reset algorithm if it's not compatible with the new key type
		const currentAlg = this.addNewEndpointForm.get('authentication.oauth2.signing_algorithm')?.value;
		if (value === 'EC' && currentAlg && !currentAlg.startsWith('ES')) {
			this.addNewEndpointForm.get('authentication.oauth2.signing_algorithm')?.setValue('ES256');
		} else if (value === 'RSA' && currentAlg && !currentAlg.startsWith('RS') && !currentAlg.startsWith('PS')) {
			this.addNewEndpointForm.get('authentication.oauth2.signing_algorithm')?.setValue('RS256');
		}
		// Update validators based on key type if client_assertion is selected
		const authType = this.selectedOAuth2AuthType || this.addNewEndpointForm.get('authentication.oauth2.authentication_type')?.value;
		if (authType === 'client_assertion') {
			this.updateJWKValidators('client_assertion', value);
		}
	}

	updateJWKValidators(authType: string | null, keyType: string | null) {
		// Get all JWK field controls
		const ecControls = {
			x: this.addNewEndpointForm.get('authentication.oauth2.signing_key.x'),
			y: this.addNewEndpointForm.get('authentication.oauth2.signing_key.y'),
			d: this.addNewEndpointForm.get('authentication.oauth2.signing_key.d'),
			crv: this.addNewEndpointForm.get('authentication.oauth2.signing_key.crv')
		};
		const rsaControls = {
			n: this.addNewEndpointForm.get('authentication.oauth2.signing_key.n'),
			e: this.addNewEndpointForm.get('authentication.oauth2.signing_key.e'),
			d: this.addNewEndpointForm.get('authentication.oauth2.signing_key.d'),
			p: this.addNewEndpointForm.get('authentication.oauth2.signing_key.p'),
			q: this.addNewEndpointForm.get('authentication.oauth2.signing_key.q'),
			dp: this.addNewEndpointForm.get('authentication.oauth2.signing_key.dp'),
			dq: this.addNewEndpointForm.get('authentication.oauth2.signing_key.dq'),
			qi: this.addNewEndpointForm.get('authentication.oauth2.signing_key.qi')
		};
		const kidControl = this.addNewEndpointForm.get('authentication.oauth2.signing_key.kid');

		if (authType === 'shared_secret' || !authType) {
			// Clear all JWK validators
			kidControl?.clearValidators();
			kidControl?.updateValueAndValidity();
			Object.values(ecControls).forEach(control => {
				control?.clearValidators();
				control?.updateValueAndValidity();
			});
			Object.values(rsaControls).forEach(control => {
				control?.clearValidators();
				control?.updateValueAndValidity();
			});
		} else if (authType === 'client_assertion') {
			// Always require kid
			kidControl?.setValidators([Validators.required]);
			kidControl?.updateValueAndValidity();

			if (keyType === 'EC') {
				// Require EC fields, clear RSA fields
				ecControls.x?.setValidators([Validators.required]);
				ecControls.x?.updateValueAndValidity();
				ecControls.y?.setValidators([Validators.required]);
				ecControls.y?.updateValueAndValidity();
				ecControls.d?.setValidators([Validators.required]);
				ecControls.d?.updateValueAndValidity();
				ecControls.crv?.setValidators([Validators.required]);
				ecControls.crv?.updateValueAndValidity();
				// Clear RSA validators
				Object.values(rsaControls).forEach(control => {
					control?.clearValidators();
					control?.updateValueAndValidity();
				});
			} else if (keyType === 'RSA') {
				// Require RSA fields, clear EC fields
				rsaControls.n?.setValidators([Validators.required]);
				rsaControls.n?.updateValueAndValidity();
				rsaControls.e?.setValidators([Validators.required]);
				rsaControls.e?.updateValueAndValidity();
				rsaControls.d?.setValidators([Validators.required]);
				rsaControls.d?.updateValueAndValidity();
				rsaControls.p?.setValidators([Validators.required]);
				rsaControls.p?.updateValueAndValidity();
				rsaControls.q?.setValidators([Validators.required]);
				rsaControls.q?.updateValueAndValidity();
				rsaControls.dp?.setValidators([Validators.required]);
				rsaControls.dp?.updateValueAndValidity();
				rsaControls.dq?.setValidators([Validators.required]);
				rsaControls.dq?.updateValueAndValidity();
				rsaControls.qi?.setValidators([Validators.required]);
				rsaControls.qi?.updateValueAndValidity();
				// Clear EC validators
				Object.values(ecControls).forEach(control => {
					control?.clearValidators();
					control?.updateValueAndValidity();
				});
			} else {
				// No key type selected yet, clear all
				Object.values(ecControls).forEach(control => {
					control?.clearValidators();
					control?.updateValueAndValidity();
				});
				Object.values(rsaControls).forEach(control => {
					control?.clearValidators();
					control?.updateValueAndValidity();
				});
			}
		}
	}

	getSigningAlgorithmOptions(): Array<{ uid: string; name: string }> {
		if (this.selectedKeyType === 'RSA') {
			return [
				{ uid: 'RS256', name: 'RS256' },
				{ uid: 'RS384', name: 'RS384' },
				{ uid: 'RS512', name: 'RS512' },
				{ uid: 'PS256', name: 'PS256' },
				{ uid: 'PS384', name: 'PS384' },
				{ uid: 'PS512', name: 'PS512' }
			];
		} else {
			return [
				{ uid: 'ES256', name: 'ES256' },
				{ uid: 'ES384', name: 'ES384' },
				{ uid: 'ES512', name: 'ES512' }
			];
		}
	}

	updateOAuth2Validators(authType: string) {
		const oauth2UrlControl = this.addNewEndpointForm.get('authentication.oauth2.url');
		const oauth2ClientIdControl = this.addNewEndpointForm.get('authentication.oauth2.client_id');
		
		if (authType === 'oauth2' && this.licenseService.hasLicense('OAuth2EndpointAuth')) {
			// Only require OAuth2 fields when OAuth2 is selected and user has license
			oauth2UrlControl?.setValidators([Validators.required]);
			oauth2ClientIdControl?.setValidators([Validators.required]);
		} else {
			// Clear validators when OAuth2 is not selected or user doesn't have license
			oauth2UrlControl?.clearValidators();
			oauth2ClientIdControl?.clearValidators();
		}
		
		oauth2UrlControl?.updateValueAndValidity({ emitEvent: false });
		oauth2ClientIdControl?.updateValueAndValidity({ emitEvent: false });
	}

	getOrganisationId() {
		const org = localStorage.getItem('CONVOY_ORG');
		if (org) {
			const organisationDetails = JSON.parse(org);
			this.organisationId = organisationDetails.uid;
		}
	}

	async checkMTLSFeatureFlag() {
		if (!this.organisationId) return;
		try {
			this.mtlsFeatureEnabled = await this.settingsService.checkFeatureFlagEnabled({
				org_id: this.organisationId,
				feature_key: 'mtls'
			});
		} catch (error) {
			this.mtlsFeatureEnabled = false;
		}
	}

	async checkOAuth2FeatureFlag() {
		if (!this.organisationId) return;
		try {
			this.oauth2FeatureEnabled = await this.settingsService.checkFeatureFlagEnabled({
				org_id: this.organisationId,
				feature_key: 'oauth-token-exchange'
			});
		} catch (error) {
			this.oauth2FeatureEnabled = false;
		}
	}
	async runEndpointValidation() {
		const authType = this.selectedAuthType || this.addNewEndpointForm.get('authentication.type')?.value || 'api_key';
		
		const configFields: any = {
			http_timeout: ['http_timeout'],
			signature: ['advanced_signatures'],
			rate_limit: ['rate_limit', 'rate_limit_duration'],
			alert_config: ['support_email', 'slack_webhook_url'],
			auth: authType === 'api_key' ? ['authentication.api_key.header_name', 'authentication.api_key.header_value'] : [],
			mtls: []
		};
		this.configurations.forEach(config => {
			const fields = configFields[config.uid];
			if (this.showConfig(config.uid)) {
				fields?.forEach((item: string) => {
					this.addNewEndpointForm.get(item)?.addValidators(Validators.required);
					this.addNewEndpointForm.get(item)?.updateValueAndValidity();
				});
			} else {
				fields?.forEach((item: string) => {
					this.addNewEndpointForm.get(item)?.removeValidators(Validators.required);
					this.addNewEndpointForm.get(item)?.updateValueAndValidity();
				});
			}
		});
		
		// Also remove validators from API key fields if OAuth2 is selected
		if (authType === 'oauth2') {
			this.addNewEndpointForm.get('authentication.api_key.header_name')?.removeValidators(Validators.required);
			this.addNewEndpointForm.get('authentication.api_key.header_name')?.updateValueAndValidity();
			this.addNewEndpointForm.get('authentication.api_key.header_value')?.removeValidators(Validators.required);
			this.addNewEndpointForm.get('authentication.api_key.header_value')?.updateValueAndValidity();
		}
		
		return;
	}

	async saveEndpoint() {
		// Prevent multiple simultaneous saves
		if (this.savingEndpoint) {
			return;
		}

		await this.runEndpointValidation();

		// Check if OAuth2 is selected and test is required (only for new endpoints, not when editing)
		if (this.selectedAuthType === 'oauth2' && this.licenseService.hasLicense('OAuth2EndpointAuth') && !this.isUpdateAction && !this.editMode) {
			if (!this.oauth2TestResult || !this.oauth2TestResult.success) {
				this.generalService.showNotification({ 
					message: 'Please test your OAuth2 connection before saving', 
					style: 'error' 
				});
				return;
			}
		}

		if (this.addNewEndpointForm.invalid) {
			this.addNewEndpointForm.markAllAsTouched();
			return;
		}


        let rateLimitDeleted = !this.showConfig('rate_limit') && this.configDeleted('rate_limit');
        if (rateLimitDeleted) {
            const configKeys = ['rate_limit', 'rate_limit_duration'];
            configKeys.forEach((key) => {
                this.addNewEndpointForm.value[key] = 0; // element type = number
                this.addNewEndpointForm.get(`${key}`)?.patchValue(0);
            });
            this.setConfigFormDeleted('rate_limit', false);
        }


		this.savingEndpoint = true;
		const endpointValue = structuredClone(this.addNewEndpointForm.value);

		// Ensure content_type is always included with a default value
		if (!endpointValue.content_type) {
			endpointValue.content_type = 'application/json';
		}

		// Clean up authentication based on type
		const authType = endpointValue.authentication?.type || 'api_key';
		if (authType === 'api_key') {
			if (!endpointValue.authentication?.api_key?.header_name && !endpointValue.authentication?.api_key?.header_value) {
				delete endpointValue.authentication;
			} else {
				// Remove oauth2 if api_key is selected
				delete endpointValue.authentication.oauth2;
			}
		} else if (authType === 'oauth2') {
			// Remove api_key if oauth2 is selected
			delete endpointValue.authentication.api_key;
			
			// Clean up oauth2 based on authentication_type
			const oauth2AuthType = endpointValue.authentication.oauth2?.authentication_type;
			if (oauth2AuthType === 'shared_secret') {
				// Remove client assertion fields
				delete endpointValue.authentication.oauth2.signing_key;
				delete endpointValue.authentication.oauth2.signing_algorithm;
				delete endpointValue.authentication.oauth2.issuer;
				delete endpointValue.authentication.oauth2.subject;
			} else if (oauth2AuthType === 'client_assertion') {
				// Remove shared secret
				delete endpointValue.authentication.oauth2.client_secret;
				
				// Clean up signing_key based on key type
				if (endpointValue.authentication.oauth2.signing_key) {
					const keyType = endpointValue.authentication.oauth2.signing_key.kty;
					if (keyType === 'EC') {
						// Remove RSA-specific fields
						delete endpointValue.authentication.oauth2.signing_key.n;
						delete endpointValue.authentication.oauth2.signing_key.e;
						delete endpointValue.authentication.oauth2.signing_key.p;
						delete endpointValue.authentication.oauth2.signing_key.q;
						delete endpointValue.authentication.oauth2.signing_key.dp;
						delete endpointValue.authentication.oauth2.signing_key.dq;
						delete endpointValue.authentication.oauth2.signing_key.qi;
					} else if (keyType === 'RSA') {
						// Remove EC-specific fields
						delete endpointValue.authentication.oauth2.signing_key.x;
						delete endpointValue.authentication.oauth2.signing_key.y;
						delete endpointValue.authentication.oauth2.signing_key.crv;
					}
				}
			}

			// Clean up field_mapping if all fields are default values
			if (endpointValue.authentication.oauth2?.field_mapping) {
				const mapping = endpointValue.authentication.oauth2.field_mapping;
				if (mapping.access_token === 'access_token' && 
					mapping.token_type === 'token_type' && 
					mapping.expires_in === 'expires_in') {
					delete endpointValue.authentication.oauth2.field_mapping;
				}
			}

			// Clean up expiry_time_unit if default
			if (endpointValue.authentication.oauth2?.expiry_time_unit === 'seconds') {
				delete endpointValue.authentication.oauth2.expiry_time_unit;
			}
		}

        // Remove mTLS config if all fields are empty or if client_key is redacted placeholder
        const mtls = this.addNewEndpointForm.value.mtls_client_cert;
        if (!mtls?.client_cert && !mtls?.client_key) {
            delete endpointValue.mtls_client_cert;
        } else if (mtls?.client_key === '[REDACTED]') {
            // Don't send mTLS config if it contains the redacted placeholder
            // (user is updating other fields but not changing the mTLS cert)
            delete endpointValue.mtls_client_cert;
        }

		try {
			const response =
				(this.isUpdateAction || this.editMode) && this.type !== 'subscription' ? await this.createEndpointService.editEndpoint({ endpointId: this.endpointUid || '', body: endpointValue }) : await this.createEndpointService.addNewEndpoint({ body: endpointValue });
			this.generalService.showNotification({ message: response.message, style: 'success' });
			this.onAction.emit({ action: this.endpointUid && this.editMode ? 'update' : 'save', data: response.data });
			this.addNewEndpointForm.reset();
			this.endpointCreated = true;
			this.savingEndpoint = false;
			return response;
		} catch (error: any) {
			this.endpointCreated = false;
			this.savingEndpoint = false;
			const errorMessage = error?.error?.message || error?.message || 'Failed to save endpoint. Please try again.';
			this.generalService.showNotification({ message: errorMessage, style: 'error' });
			console.error('Error saving endpoint:', error);
			return;
		}
	}

	async getEndpointDetails() {
		this.isLoadingEndpointDetails = true;

		try {
			const response = await this.endpointService.getEndpoint(this.endpointUid);
			const endpointDetails: ENDPOINT = response.data;

			this.endpointSecret = endpointDetails?.secrets?.find(secret => !secret.expires_at);
			if (endpointDetails.rate_limit_duration) this.toggleConfigForm('rate_limit');
		this.addNewEndpointForm.patchValue(endpointDetails);
		
		// Update selectedAuthType after patching form
		this.selectedAuthType = endpointDetails.authentication?.type || 'api_key';
		
		// Update selectedOAuth2AuthType if OAuth2 is configured
		if (endpointDetails.authentication?.oauth2) {
			this.selectedOAuth2AuthType = endpointDetails.authentication.oauth2.authentication_type || 'shared_secret';
			// Update selectedKeyType if signing_key is configured
			if (endpointDetails.authentication.oauth2.signing_key?.kty) {
				this.selectedKeyType = endpointDetails.authentication.oauth2.signing_key.kty || 'EC';
				// Update validators based on key type
				this.updateJWKValidators('client_assertion', this.selectedKeyType);
			}
			// Set test result as successful for existing endpoints (they were tested when created)
			this.oauth2TestResult = { success: true, message: 'OAuth2 configuration loaded from existing endpoint' };
		}

		// Set content type and toggle the configuration if it's not the default
		if (endpointDetails.content_type && endpointDetails.content_type !== 'application/json') {
			this.selectedContentType = endpointDetails.content_type;
			this.toggleConfigForm('content_type');
		}

		if (endpointDetails.owner_id) this.toggleConfigForm('owner_id');

			if (endpointDetails.support_email) this.toggleConfigForm('alert_config');
			if (endpointDetails.authentication?.api_key?.header_value || endpointDetails.authentication?.api_key?.header_name || endpointDetails.authentication?.oauth2) {
				this.toggleConfigForm('auth');
			}
			if (endpointDetails.http_timeout) this.toggleConfigForm('http_timeout');
			if (endpointDetails.mtls_client_cert) this.toggleConfigForm('mtls');

			this.isLoadingEndpointDetails = false;
		} catch {
			this.isLoadingEndpointDetails = false;
		}
	}

	async getEndpoints() {
		this.isLoadingEndpoints = true;
		try {
			const response = await this.privateService.getEndpoints();
			const endpoints = response.data.content;
			if (endpoints.length > 0 && this.router.url.includes('/configure')) this.onAction.emit({ action: 'save' });
			this.isLoadingEndpoints = false;
		} catch {
			this.isLoadingEndpoints = false;
		}
	}

	getDurationInSeconds(timeString: string) {
		const timeParts = timeString.split('m');
		let minutes = 0;
		let seconds = 0;

		if (timeParts.length > 0) {
			minutes = parseInt(timeParts[0], 10);
		}

		if (timeParts.length > 1) {
			seconds = parseInt(timeParts[1].replace('s', ''), 10);
		}
		const totalSeconds = minutes * 60 + seconds;

		return totalSeconds;
	}

	toggleConfigForm(configValue: string, deleted?: boolean) {
		this.configurations.forEach(config => {
			if (config.uid === configValue) {
                config.show = !config.show;
                config.deleted = deleted ?? false;
            }
		});

		// When toggling content_type, ensure the form control has the correct value
		if (configValue === 'content_type' && !deleted) {
			// Give Angular time to render the select component, then set the value
			setTimeout(() => {
				const currentValue = this.addNewEndpointForm.get('content_type')?.value;
				if (!currentValue || currentValue === '') {
					this.addNewEndpointForm.patchValue({ content_type: 'application/json' });
				}
			}, 0);
		}
	}

    setConfigFormDeleted(configValue: string, deleted: boolean) {
        this.configurations.forEach(config => {
            if (config.uid === configValue) {
                config.deleted = deleted;
            }
        });
    }

	showConfig(configValue: string): boolean {
		const config = this.configurations.find(config => config.uid === configValue);
		if (!config) return false;
		
		// For mTLS, also check if feature flag is enabled
		if (configValue === 'mtls' && !this.mtlsFeatureEnabled) {
			return false;
		}
		
		return config.show || false;
	}

	onContentTypeSelected(value: any) {
		this.addNewEndpointForm.get('content_type')?.setValue(value);
	}

    configDeleted(configValue: string): boolean {
        return this.configurations.find(config => config.uid === configValue)?.deleted || false;
    }

	get shouldShowBorder(): number {
		return this.configurations.filter(config => config.show).length;
	}

	get isUpdateAction(): boolean {
		return this.endpointUid && this.endpointUid !== 'new' && this.currentRoute !== 'setup';
	}

	async testOAuth2Connection() {
		const oauth2Form = this.addNewEndpointForm.get('authentication.oauth2');
		if (!oauth2Form || oauth2Form.invalid) {
			oauth2Form?.markAllAsTouched();
			this.generalService.showNotification({ message: 'Please fill all required OAuth2 fields', style: 'error' });
			return;
		}

		this.testingOAuth2 = true;
		this.oauth2TestResult = null;

		try {
			const oauth2Value = structuredClone(oauth2Form.value);
			
			// Clean up based on authentication_type
			const authType = oauth2Value.authentication_type;
			if (authType === 'shared_secret') {
				delete oauth2Value.signing_key;
				delete oauth2Value.signing_algorithm;
				delete oauth2Value.issuer;
				delete oauth2Value.subject;
			} else if (authType === 'client_assertion') {
				delete oauth2Value.client_secret;
				
				// Clean up signing_key based on key type
				if (oauth2Value.signing_key) {
					const keyType = oauth2Value.signing_key.kty;
					if (keyType === 'EC') {
						// Remove RSA-specific fields
						delete oauth2Value.signing_key.n;
						delete oauth2Value.signing_key.e;
						delete oauth2Value.signing_key.p;
						delete oauth2Value.signing_key.q;
						delete oauth2Value.signing_key.dp;
						delete oauth2Value.signing_key.dq;
						delete oauth2Value.signing_key.qi;
					} else if (keyType === 'RSA') {
						// Remove EC-specific fields
						delete oauth2Value.signing_key.x;
						delete oauth2Value.signing_key.y;
						delete oauth2Value.signing_key.crv;
					}
				}
			}

			// Clean up field_mapping if all fields are default values
			if (oauth2Value.field_mapping) {
				const mapping = oauth2Value.field_mapping;
				if (mapping.access_token === 'access_token' && 
					mapping.token_type === 'token_type' && 
					mapping.expires_in === 'expires_in') {
					delete oauth2Value.field_mapping;
				}
			}

			// Clean up expiry_time_unit if default
			if (oauth2Value.expiry_time_unit === 'seconds') {
				delete oauth2Value.expiry_time_unit;
			}

			const response = await this.createEndpointService.testOAuth2Connection({ oauth2: oauth2Value });
			this.oauth2TestResult = response.data;
			
			if (response.data.success) {
				this.generalService.showNotification({ 
					message: 'OAuth2 connection test successful!', 
					style: 'success' 
				});
			} else {
				this.generalService.showNotification({ 
					message: `OAuth2 connection test failed: ${response.data.error}`, 
					style: 'error' 
				});
			}
		} catch (error: any) {
			this.oauth2TestResult = {
				success: false,
				error: error?.error?.message || error?.message || 'Failed to test OAuth2 connection'
			};
			this.generalService.showNotification({ 
				message: `OAuth2 connection test failed: ${this.oauth2TestResult.error}`, 
				style: 'error' 
			});
		} finally {
			this.testingOAuth2 = false;
		}
	}
}
