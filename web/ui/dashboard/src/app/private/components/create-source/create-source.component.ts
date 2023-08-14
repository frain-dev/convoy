import { Component, EventEmitter, Input, OnInit, Output, inject } from '@angular/core';
import { FormBuilder, FormGroup, Validators } from '@angular/forms';
import { ActivatedRoute, Router } from '@angular/router';
import { SOURCE } from 'src/app/models/source.model';
import { GeneralService } from 'src/app/services/general/general.service';
import { PrivateService } from '../../private.service';
import { CreateSourceService } from './create-source.service';
import { RbacService } from 'src/app/services/rbac/rbac.service';

@Component({
	selector: 'convoy-create-source',
	templateUrl: './create-source.component.html',
	styleUrls: ['./create-source.component.scss']
})
export class CreateSourceComponent implements OnInit {
	@Input('action') action: 'update' | 'create' = 'create';
	@Input('showAction') showAction: 'true' | 'false' = 'false';
	@Output() onAction = new EventEmitter<any>();
	sourceForm: FormGroup = this.formBuilder.group({
		name: ['', Validators.required],
		is_disabled: [true, Validators.required],
		type: ['', Validators.required],
		custom_response: this.formBuilder.group({
			body: [''],
			content_type: ['']
		}),
		idempotency_keys: [null],
		verifier: this.formBuilder.group({
			api_key: this.formBuilder.group({
				header_name: [''],
				header_value: ['']
			}),
			basic_auth: this.formBuilder.group({
				password: [''],
				username: ['']
			}),
			hmac: this.formBuilder.group({
				encoding: [''],
				hash: [''],
				header: [''],
				secret: ['']
			}),
			type: ['', Validators.required]
		}),
		pub_sub: this.formBuilder.group({
			type: [''],
			workers: [null],
			google: this.formBuilder.group({
				service_account: [''],
				subscription_id: [''],
				project_id: ['']
			}),
			sqs: this.formBuilder.group({
				queue_name: [''],
				access_key_id: [''],
				secret_key: [''],
				default_region: ['']
			}),
			kafka: this.formBuilder.group({
				brokers: [null],
				consumer_group_id: [null],
				topic_name: [null],
				auth: this.formBuilder.group({
					type: [null],
					tls: [null],
					username: [null],
					password: [null],
					hash: [null]
				})
			})
		})
	});
	authTypes = ['plain', 'scram'];
	sourceTypes = [
		{ value: 'http', viewValue: 'Ingestion HTTP', description: 'Trigger webhook event from a thirdparty webhook event' },
		{ value: 'pub_sub', viewValue: 'Pub/Sub (Coming Soon)', description: 'Trigger webhook event from your Pub/Sub messaging system' },
		{ value: 'db_change_stream', viewValue: 'DB Change Stream (Coming Soon)', description: 'Trigger webhook event from your DB change stream' }
	];
	pubSubTypes = [
		{ uid: 'google', name: 'Google Pub/Sub' },
		{ uid: 'kafka', name: 'Kafka Pub/Sub' },
		{ uid: 'sqs', name: 'AWS SQS' }
	];
	httpTypes = [
		{ value: 'noop', viewValue: 'None' },
		{ value: 'hmac', viewValue: 'HMAC' },
		{ value: 'basic_auth', viewValue: 'Basic Auth' },
		{ value: 'api_key', viewValue: 'API Key' },
		{ value: 'github', viewValue: 'Github' },
		{ value: 'twitter', viewValue: 'Twitter' },
		{ value: 'shopify', viewValue: 'Shopify' }
	];
	encodings = ['base64', 'hex'];
	hashAlgorithms = ['SHA256', 'SHA512'];

	AWSregions = [
		{ uid: 'us-east-2', name: 'US East (Ohio)' },
		{ uid: 'us-east-1', name: 'US East (N. Virginia)' },
		{ uid: 'us-west-1', name: 'US West (N. California)' },
		{ uid: 'us-west-2', name: 'US West (Oregon)' },
		{ uid: 'af-south-1', name: 'Africa (Cape Town)' },
		{ uid: 'ap-east-1', name: 'Asia Pacific (Hong Kong)' },
		{ uid: 'ap-south-2', name: 'Asia Pacific (Hyderabad)' },
		{ uid: 'ap-southeast-3', name: 'Asia Pacific (Jakarta)' },
		{ uid: 'ap-southeast-4', name: 'Asia Pacific (Melbourne)' },
		{ uid: 'ap-south-1', name: 'Asia Pacific (Mumbai)' },
		{ uid: 'ap-northeast-3', name: 'Asia Pacific (Osaka)' },
		{ uid: 'ap-northeast-2', name: 'Asia Pacific (Seoul)' },
		{ uid: 'ap-southeast-1', name: 'Asia Pacific (Singapore)' },
		{ uid: 'ap-southeast-2', name: 'Asia Pacific (Sydney)' },
		{ uid: 'ap-northeast-1', name: 'Asia Pacific (Tokyo)' },
		{ uid: 'ca-central-1', name: 'Canada (Central)' },
		{ uid: 'eu-central-1', name: 'Europe (Frankfurt)' },
		{ uid: 'eu-west-1', name: 'Europe (Ireland)' },
		{ uid: 'eu-west-2', name: 'Europe (London)' },
		{ uid: 'eu-south-1', name: 'Europe (Milan)' },
		{ uid: 'eu-west-3', name: 'Europe (Paris)' },
		{ uid: 'eu-south-2', name: 'Europe (Spain)' },
		{ uid: 'eu-north-1', name: 'Europe (Stockholm)' },
		{ uid: 'eu-central-2', name: 'Europe (Zurich)' },
		{ uid: 'me-south-1', name: 'Middle East (Bahrain)' },
		{ uid: 'me-central-1', name: 'Middle East (UAE)' },
		{ uid: 'sa-east-1', name: 'South America (São Paulo)' },
		{ uid: 'us-gov-east-1', name: 'AWS GovCloud (US-East)' },
		{ uid: 'us-gov-west-1', name: 'AWS GovCloud (US-West)' }
	];

	preConfiguredSources: ['github', 'shopify', 'twitter'] = ['github', 'shopify', 'twitter'];
	sourceVerifications = [
		{ uid: 'noop', name: 'None' },
		{ uid: 'hmac', name: 'HMAC' },
		{ uid: 'basic_auth', name: 'Basic Auth' },
		{ uid: 'api_key', name: 'API Key' },
		{ uid: 'github', name: 'Github' },
		{ uid: 'twitter', name: 'Twitter' },
		{ uid: 'shopify', name: 'Shopify' }
	];
	sourceId = this.route.snapshot.params.id;
	isloading = false;
	confirmModal = false;
	addKafkaAuthentication = false;
	sourceDetails!: SOURCE;
	sourceCreated: boolean = false;
	showSourceUrl = false;
	sourceData!: SOURCE;
	configurations = [
		{ uid: 'custom_response', name: 'Custom Response', show: false },
		{ uid: 'idempotency', name: 'Idempotency', show: false }
	];

	brokerAddresses: string[] = [];
	private rbacService = inject(RbacService);

	constructor(private formBuilder: FormBuilder, private createSourceService: CreateSourceService, public privateService: PrivateService, private route: ActivatedRoute, private router: Router, private generalService: GeneralService) {}

	async ngOnInit() {
		if (this.action === 'update') this.getSourceDetails();
		this.privateService.getProjectDetails?.type === 'incoming' ? this.sourceForm.patchValue({ type: 'http' }) : this.sourceForm.patchValue({ type: 'pub_sub' });

		if (!(await this.rbacService.userCanAccess('Sources|MANAGE'))) this.sourceForm.disable();
	}

	async getSourceDetails() {
		this.isloading = true;
		try {
			const response = await this.createSourceService.getSourceDetails(this.sourceId);
			this.sourceDetails = response.data;
			const sourceProvider = response.data?.provider;

			this.sourceForm.patchValue(response.data);
			if (this.sourceDetails.custom_response.body || this.sourceDetails.custom_response.content_type) this.toggleConfigForm('custom_response');
			if (this.sourceDetails.idempotency_keys.length) this.toggleConfigForm('idempotency');

			if (this.isCustomSource(sourceProvider)) this.sourceForm.patchValue({ verifier: { type: sourceProvider } });

			if (response.data.pub_sub.kafka.auth.type) this.addKafkaAuthentication = true;
			if (response.data.pub_sub.kafka.brokers) this.brokerAddresses = response.data.pub_sub.kafka.brokers;
			this.isloading = false;

			return;
		} catch (error) {
			this.isloading = false;
			return error;
		}
	}

	checkSourceSetup() {
		if (this.privateService.getProjectDetails?.type === 'incoming') {
			delete this.sourceForm.value.pub_sub;
			const verifierType = this.sourceForm.get('verifier.type')?.value;
			const verifier = this.isCustomSource(verifierType) ? 'hmac' : verifierType;

			if (this.sourceForm.get('verifier.type')?.value === 'github') this.sourceForm.get('verifier.hmac')?.patchValue({ encoding: 'hex', header: 'X-Hub-Signature-256', hash: 'SHA256' });
			if (this.sourceForm.get('verifier.type')?.value === 'shopify') this.sourceForm.get('verifier.hmac')?.patchValue({ encoding: 'base64', header: 'X-Shopify-Hmac-SHA256', hash: 'SHA256' });
			if (this.sourceForm.get('verifier.type')?.value === 'twitter') this.sourceForm.get('verifier.hmac')?.patchValue({ encoding: 'base64', header: 'X-Twitter-Webhooks-Signature', hash: 'SHA256' });
			return {
				...this.sourceForm.value,
				provider: this.isCustomSource(verifierType) ? verifierType : '',
				verifier: {
					type: verifier,
					[verifier]: { ...this.sourceForm.get('verifier.' + verifier)?.value }
				}
			};
		} else {
			delete this.sourceForm.value.verifier;
			const pubSubType = this.sourceForm.get('pub_sub.type')?.value;
			if (pubSubType === 'google') {
				delete this.sourceForm.value.pub_sub.sqs;
			} else delete this.sourceForm.value.pub_sub.google;
			return this.sourceForm.value;
		}
	}

	parseJsonFile(event: any) {
		const fileReader = new FileReader();
		fileReader.readAsText(event, 'UTF-8');
		fileReader.onload = () => {
			if (fileReader.result)
				this.sourceForm.patchValue({
					pub_sub: {
						google: {
							service_account: btoa(fileReader.result.toString())
						}
					}
				});
		};
		fileReader.onerror = error => {
			this.generalService.showNotification({ message: 'Please upload a JSON file', style: 'warning' });
		};
	}

	deleteJsonFile() {
		if (this.action === 'create') this.sourceForm.value.pub_sub.google.service_account = null;
		else
			this.sourceForm.patchValue({
				pub_sub: {
					google: {
						service_account: this.sourceDetails.pub_sub.google.service_account
					}
				}
			});
	}

	async saveSource() {
		const sourceData = this.checkSourceSetup();
		await this.runSourceFormValidation();
		if (!this.sourceForm.valid) {
			this.isloading = false;
			return this.sourceForm.markAllAsTouched();
		}
		this.isloading = true;

		try {
			const response = this.action === 'update' ? await this.createSourceService.updateSource({ data: sourceData, id: this.sourceId }) : await this.createSourceService.createSource({ sourceData });
			document.getElementById('configureProjectForm')?.scroll({ top: 0, behavior: 'smooth' });
			this.sourceData = response.data;
			this.onAction.emit({ action: this.action, data: response.data });
			this.sourceCreated = true;
			return response;
		} catch (error) {
			this.sourceCreated = false;
			this.isloading = false;
		}
	}

	async getSources() {
		this.isloading = true;
		try {
			const response = await this.privateService.getSources();
			const sources = response.data.content;
			if (sources.length > 0 && this.router.url.includes('/configure')) this.onAction.emit({ action: 'create' });
			this.isloading = false;
		} catch (error) {
			this.isloading = false;
		}
	}

	isCustomSource(sourceValue: string): boolean {
		const customSources = ['github', 'twitter', 'shopify'];
		const checkForCustomSource = customSources.some(source => sourceValue.includes(source));

		return checkForCustomSource;
	}

	toggleConfigForm(configValue: string, value?: boolean) {
		this.configurations.forEach(config => {
			if (config.uid === configValue) config.show = value ? value : !config.show;
		});
	}

	showConfig(configValue: string): boolean {
		return this.configurations.find(config => config.uid === configValue)?.show || false;
	}

	setRegionValue(value: any) {
		this.sourceForm.get('pub_sub.sqs')?.patchValue({ default_region: value });
	}

	async runSourceFormValidation() {
		if (this.configurations[0].show) {
			this.sourceForm.get('custom_response.body')?.addValidators(Validators.required);
			this.sourceForm.get('custom_response.body')?.updateValueAndValidity();
			this.sourceForm.get('custom_response.content_type')?.addValidators(Validators.required);
			this.sourceForm.get('custom_response.content_type')?.updateValueAndValidity();
		} else {
			this.sourceForm.get('custom_response.body')?.removeValidators(Validators.required);
			this.sourceForm.get('custom_response.body')?.updateValueAndValidity();
			this.sourceForm.get('custom_response.content_type')?.removeValidators(Validators.required);
			this.sourceForm.get('custom_response.content_type')?.updateValueAndValidity();
		}

		if (this.privateService.getProjectDetails?.type === 'incoming') {
			this.sourceForm.get('verifier.type')?.addValidators(Validators.required);
			this.sourceForm.get('verifier.type')?.updateValueAndValidity();

			if (this.sourceForm.get('verifier')?.value.type === 'api_key') {
				this.sourceForm.get('verifier.api_key.header_name')?.addValidators(Validators.required);
				this.sourceForm.get('verifier.api_key.header_value')?.addValidators(Validators.required);
				this.sourceForm.get('verifier.api_key.header_name')?.updateValueAndValidity();
				this.sourceForm.get('verifier.api_key.header_value')?.updateValueAndValidity();
			} else {
				this.sourceForm.get('verifier.api_key.header_name')?.removeValidators(Validators.required);
				this.sourceForm.get('verifier.api_key.header_value')?.removeValidators(Validators.required);
				this.sourceForm.get('verifier.api_key.header_name')?.updateValueAndValidity();
				this.sourceForm.get('verifier.api_key.header_value')?.updateValueAndValidity();
			}

			if (this.sourceForm.get('verifier')?.value.type === 'basic_auth') {
				this.sourceForm.get('verifier.basic_auth.password')?.addValidators(Validators.required);
				this.sourceForm.get('verifier.basic_auth.username')?.addValidators(Validators.required);
				this.sourceForm.get('verifier.basic_auth.password')?.updateValueAndValidity();
				this.sourceForm.get('verifier.basic_auth.username')?.updateValueAndValidity();
			} else {
				this.sourceForm.get('verifier.basic_auth.password')?.removeValidators(Validators.required);
				this.sourceForm.get('verifier.basic_auth.username')?.removeValidators(Validators.required);
				this.sourceForm.get('verifier.basic_auth.password')?.updateValueAndValidity();
				this.sourceForm.get('verifier.basic_auth.username')?.updateValueAndValidity();
			}

			if (this.sourceForm.get('verifier')?.value.type === 'hmac') {
				this.sourceForm.get('verifier.hmac.encoding')?.addValidators(Validators.required);
				this.sourceForm.get('verifier.hmac.hash')?.addValidators(Validators.required);
				this.sourceForm.get('verifier.hmac.header')?.addValidators(Validators.required);
				this.sourceForm.get('verifier.hmac.secret')?.addValidators(Validators.required);
				this.sourceForm.get('verifier.hmac.encoding')?.updateValueAndValidity();
				this.sourceForm.get('verifier.hmac.hash')?.updateValueAndValidity();
				this.sourceForm.get('verifier.hmac.header')?.updateValueAndValidity();
				this.sourceForm.get('verifier.hmac.secret')?.updateValueAndValidity();
			} else {
				this.sourceForm.get('verifier.hmac.encoding')?.removeValidators(Validators.required);
				this.sourceForm.get('verifier.hmac.hash')?.removeValidators(Validators.required);
				this.sourceForm.get('verifier.hmac.header')?.removeValidators(Validators.required);
				this.sourceForm.get('verifier.hmac.secret')?.removeValidators(Validators.required);
				this.sourceForm.get('verifier.hmac.encoding')?.updateValueAndValidity();
				this.sourceForm.get('verifier.hmac.hash')?.updateValueAndValidity();
				this.sourceForm.get('verifier.hmac.header')?.updateValueAndValidity();
				this.sourceForm.get('verifier.hmac.secret')?.updateValueAndValidity();
			}
		} else {
			this.sourceForm.get('verifier.type')?.removeValidators(Validators.required);
			this.sourceForm.get('verifier.type')?.updateValueAndValidity();
		}

		if (this.privateService.getProjectDetails?.type === 'outgoing') {
			this.sourceForm.get('pub_sub.workers')?.addValidators(Validators.required);
			this.sourceForm.get('pub_sub.workers')?.updateValueAndValidity();
			this.sourceForm.get('pub_sub.type')?.addValidators(Validators.required);
			this.sourceForm.get('pub_sub.type')?.updateValueAndValidity();

			if (this.sourceForm.get('pub_sub')?.value.type === 'google') {
				this.sourceForm.get('pub_sub.google.service_account')?.addValidators(Validators.required);
				this.sourceForm.get('pub_sub.google.subscription_id')?.addValidators(Validators.required);
				this.sourceForm.get('pub_sub.google.project_id')?.addValidators(Validators.required);
				this.sourceForm.get('pub_sub.google.service_account')?.updateValueAndValidity();
				this.sourceForm.get('pub_sub.google.subscription_id')?.updateValueAndValidity();
				this.sourceForm.get('pub_sub.google.project_id')?.updateValueAndValidity();
			} else {
				this.sourceForm.get('pub_sub.google.service_account')?.removeValidators(Validators.required);
				this.sourceForm.get('pub_sub.google.subscription_id')?.removeValidators(Validators.required);
				this.sourceForm.get('pub_sub.google.project_id')?.removeValidators(Validators.required);
				this.sourceForm.get('pub_sub.google.service_account')?.updateValueAndValidity();
				this.sourceForm.get('pub_sub.google.subscription_id')?.updateValueAndValidity();
				this.sourceForm.get('pub_sub.google.project_id')?.updateValueAndValidity();
			}

			if (this.sourceForm.get('pub_sub')?.value.type === 'sqs') {
				this.sourceForm.get('pub_sub.sqs.queue_name')?.addValidators(Validators.required);
				this.sourceForm.get('pub_sub.sqs.access_key_id')?.addValidators(Validators.required);
				this.sourceForm.get('pub_sub.sqs.secret_key')?.addValidators(Validators.required);
				this.sourceForm.get('pub_sub.sqs.default_region')?.addValidators(Validators.required);
				this.sourceForm.get('pub_sub.sqs.queue_name')?.updateValueAndValidity();
				this.sourceForm.get('pub_sub.sqs.access_key_id')?.updateValueAndValidity();
				this.sourceForm.get('pub_sub.sqs.secret_key')?.updateValueAndValidity();
				this.sourceForm.get('pub_sub.sqs.default_region')?.updateValueAndValidity();
			} else {
				this.sourceForm.get('pub_sub.sqs.queue_name')?.removeValidators(Validators.required);
				this.sourceForm.get('pub_sub.sqs.access_key_id')?.removeValidators(Validators.required);
				this.sourceForm.get('pub_sub.sqs.secret_key')?.removeValidators(Validators.required);
				this.sourceForm.get('pub_sub.sqs.default_region')?.removeValidators(Validators.required);
				this.sourceForm.get('pub_sub.sqs.queue_name')?.updateValueAndValidity();
				this.sourceForm.get('pub_sub.sqs.access_key_id')?.updateValueAndValidity();
				this.sourceForm.get('pub_sub.sqs.secret_key')?.updateValueAndValidity();
				this.sourceForm.get('pub_sub.sqs.default_region')?.updateValueAndValidity();
			}

			if (this.sourceForm.get('pub_sub')?.value.type === 'kafka') {
				this.sourceForm.get('pub_sub.kafka.brokers')?.addValidators(Validators.required);
				this.sourceForm.get('pub_sub.kafka.consumer_group_id')?.addValidators(Validators.required);
				this.sourceForm.get('pub_sub.kafka.topic_name')?.addValidators(Validators.required);
				this.sourceForm.get('pub_sub.kafka.brokers')?.updateValueAndValidity();
				this.sourceForm.get('pub_sub.kafka.consumer_group_id')?.updateValueAndValidity();
				this.sourceForm.get('pub_sub.kafka.topic_name')?.updateValueAndValidity();
			} else {
				this.sourceForm.get('pub_sub.kafka.brokers')?.removeValidators(Validators.required);
				this.sourceForm.get('pub_sub.kafka.consumer_group_id')?.removeValidators(Validators.required);
				this.sourceForm.get('pub_sub.kafka.topic_name')?.removeValidators(Validators.required);
				this.sourceForm.get('pub_sub.kafka.brokers')?.updateValueAndValidity();
				this.sourceForm.get('pub_sub.kafka.consumer_group_id')?.updateValueAndValidity();
				this.sourceForm.get('pub_sub.kafka.topic_name')?.updateValueAndValidity();
			}

			if (this.addKafkaAuthentication) {
				this.sourceForm.get('pub_sub.kafka.auth.type')?.addValidators(Validators.required);
				this.sourceForm.get('pub_sub.kafka.auth.tls')?.addValidators(Validators.required);
				this.sourceForm.get('pub_sub.kafka.auth.username')?.addValidators(Validators.required);
				this.sourceForm.get('pub_sub.kafka.auth.password')?.addValidators(Validators.required);
				this.sourceForm.get('pub_sub.kafka.auth.type')?.updateValueAndValidity();
				this.sourceForm.get('pub_sub.kafka.auth.tls')?.updateValueAndValidity();
				this.sourceForm.get('pub_sub.kafka.auth.username')?.updateValueAndValidity();
				this.sourceForm.get('pub_sub.kafka.auth.password')?.updateValueAndValidity();
			} else {
				this.sourceForm.get('pub_sub.kafka.auth.type')?.removeValidators(Validators.required);
				this.sourceForm.get('pub_sub.kafka.auth.tls')?.removeValidators(Validators.required);
				this.sourceForm.get('pub_sub.kafka.auth.username')?.removeValidators(Validators.required);
				this.sourceForm.get('pub_sub.kafka.auth.password')?.removeValidators(Validators.required);
				this.sourceForm.get('pub_sub.kafka.auth.type')?.updateValueAndValidity();
				this.sourceForm.get('pub_sub.kafka.auth.tls')?.updateValueAndValidity();
				this.sourceForm.get('pub_sub.kafka.auth.username')?.updateValueAndValidity();
				this.sourceForm.get('pub_sub.kafka.auth.password')?.updateValueAndValidity();
			}
		} else {
			this.sourceForm.get('pub_sub.workers')?.removeValidators(Validators.required);
			this.sourceForm.get('pub_sub.workers')?.updateValueAndValidity();
			this.sourceForm.get('pub_sub.type')?.removeValidators(Validators.required);
			this.sourceForm.get('pub_sub.type')?.updateValueAndValidity();
		}

		return;
	}

	addBrokers(brokers: string[]) {
		this.sourceForm.patchValue({
			pub_sub: {
				kafka: { brokers }
			}
		});
	}
}
