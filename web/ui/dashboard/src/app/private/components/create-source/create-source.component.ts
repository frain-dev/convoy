import { Component, ElementRef, EventEmitter, Input, OnInit, Output, ViewChild, inject } from '@angular/core';
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
	@ViewChild('sourceURLDialog', { static: true }) sourceURLDialog!: ElementRef<HTMLDialogElement>;
	@Input('action') action: 'update' | 'create' = 'create';
	@Input('showAction') showAction: 'true' | 'false' = 'false';
	@Input('showModal') showModal: 'true' | 'false' = 'false';
	@Output() onAction = new EventEmitter<any>();
	sourceForm: FormGroup = this.formBuilder.group({
		name: ['', Validators.required],
		is_disabled: [true, Validators.required],
		type: ['', Validators.required],
		body_function: [null],
		header_function: [null],
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
			amqp: this.formBuilder.group({
				schema: [''],
				host: [''],
				port: [''],
				queue: [''],
				deadLetterExchange: [null],
				auth: this.formBuilder.group({
					user: [null],
					password: [null]
				}),
				bindExchange: this.formBuilder.group({
					exchange: [null],
					routingKey: ['""']
				})
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
		{ uid: 'kafka', name: 'Kafka' },
		{ uid: 'sqs', name: 'AWS SQS' },
		{ uid: 'amqp', name: 'AMQP / RabbitMQ' }
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
	addAmqpAuthentication = false;
	addAmqpQueueBinding = false;
	sourceDetails!: SOURCE;
	sourceCreated: boolean = false;
	showSourceUrl = false;
	sourceData!: SOURCE;
	configurations!: { uid: string; name: string; show: boolean }[];

	brokerAddresses: string[] = [];
	private rbacService = inject(RbacService);
	sourceURL!: string;
	showTransformDialog = false;

	constructor(private formBuilder: FormBuilder, private createSourceService: CreateSourceService, public privateService: PrivateService, private route: ActivatedRoute, private router: Router, private generalService: GeneralService) {}

	async ngOnInit() {
		if (this.privateService.getProjectDetails.type === 'incoming')
			this.configurations = [
				{ uid: 'custom_response', name: 'Custom Response', show: false },
				{ uid: 'idempotency', name: 'Idempotency', show: false }
			];
		else this.configurations = [{ uid: 'tranform_config', name: 'Transform', show: false }];

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

			if (this.sourceDetails.idempotency_keys?.length) this.toggleConfigForm('idempotency');

			if (this.isCustomSource(sourceProvider)) this.sourceForm.patchValue({ verifier: { type: sourceProvider } });

			if (response.data.pub_sub.kafka.brokers) this.brokerAddresses = response.data.pub_sub.kafka.brokers;

			if (response.data.pub_sub.kafka.auth?.type) this.addKafkaAuthentication = true;

			if (response.data.pub_sub.amqp.auth?.user) this.addAmqpAuthentication = true;

			if (response.data.pub_sub.amqp.bindedExchange) this.addAmqpQueueBinding = true;

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

		if (!this.addKafkaAuthentication) delete sourceData.pub_sub?.kafka?.auth;

		this.isloading = true;

		try {
			const response = this.action === 'update' ? await this.createSourceService.updateSource({ data: sourceData, id: this.sourceId }) : await this.createSourceService.createSource({ sourceData });
			document.getElementById('configureProjectForm')?.scroll({ top: 0, behavior: 'smooth' });
			this.sourceData = response.data;
			this.sourceCreated = true;

			if (this.showModal == 'true') {
				this.sourceURL = this.sourceData.url;
				this.sourceURLDialog.nativeElement.showModal();
				return response;
			}

			this.onAction.emit({ action: this.action, data: this.sourceData });
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
		return this.configurations?.find(config => config.uid === configValue)?.show || false;
	}

	setRegionValue(value: any) {
		this.sourceForm.get('pub_sub.sqs')?.patchValue({ default_region: value });
	}

	async runSourceFormValidation() {
		if (this.showConfig('custom_response')) {
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

			const verifiers: any = {
				api_key: ['verifier.api_key.header_name', 'verifier.api_key.header_value'],
				basic_auth: ['verifier.basic_auth.password', 'verifier.basic_auth.username'],
				hmac: ['verifier.hmac.encoding', 'verifier.hmac.hash', 'verifier.hmac.header', 'verifier.hmac.secret']
			};

			Object.keys(verifiers).forEach((verifier: any) => {
				const fields = verifiers[verifier];
				if (this.sourceForm.get('verifier')?.value.type === verifier) {
					fields?.forEach((item: string) => {
						this.sourceForm.get(item)?.addValidators(Validators.required);
						this.sourceForm.get(item)?.updateValueAndValidity();
					});
				} else {
					fields?.forEach((item: string) => {
						this.sourceForm.get(item)?.removeValidators(Validators.required);
						this.sourceForm.get(item)?.updateValueAndValidity();
					});
				}
			});
		} else {
			this.sourceForm.get('verifier.type')?.removeValidators(Validators.required);
			this.sourceForm.get('verifier.type')?.updateValueAndValidity();
		}

		if (this.privateService.getProjectDetails?.type === 'outgoing') {
			this.sourceForm.get('pub_sub.workers')?.addValidators(Validators.required);
			this.sourceForm.get('pub_sub.workers')?.updateValueAndValidity();
			this.sourceForm.get('pub_sub.type')?.addValidators(Validators.required);
			this.sourceForm.get('pub_sub.type')?.updateValueAndValidity();

			const pubSubs: any = {
				google: ['pub_sub.google.service_account', 'pub_sub.google.subscription_id', 'pub_sub.google.project_id'],
				sqs: ['pub_sub.sqs.queue_name', 'pub_sub.sqs.access_key_id', 'pub_sub.sqs.secret_key', 'pub_sub.sqs.default_region'],
				kafka: ['pub_sub.kafka.brokers', 'pub_sub.kafka.topic_name'],
				amqp: ['pub_sub.amqp.schema', 'pub_sub.amqp.host', 'pub_sub.amqp.port', 'pub_sub.amqp.queue', 'pub_sub_amqp.deadLetterExchange']
			};

			Object.keys(pubSubs).forEach((pubSub: any) => {
				const fields = pubSubs[pubSub];
				if (this.sourceForm.get('pub_sub')?.value.type === pubSub) {
					fields?.forEach((item: string) => {
						this.sourceForm.get(item)?.addValidators(Validators.required);
						this.sourceForm.get(item)?.updateValueAndValidity();
					});
				} else {
					fields?.forEach((item: string) => {
						this.sourceForm.get(item)?.removeValidators(Validators.required);
						this.sourceForm.get(item)?.updateValueAndValidity();
					});
				}
			});

			const kafkaAuths = ['pub_sub.kafka.auth.type', 'pub_sub.kafka.auth.tls', 'pub_sub.kafka.auth.username', 'pub_sub.kafka.auth.password'];

			if (this.addKafkaAuthentication) {
				kafkaAuths?.forEach((item: string) => {
					this.sourceForm.get(item)?.addValidators(Validators.required);
					this.sourceForm.get(item)?.updateValueAndValidity();
				});
			} else {
				kafkaAuths?.forEach((item: string) => {
					this.sourceForm.get(item)?.removeValidators(Validators.required);
					this.sourceForm.get(item)?.updateValueAndValidity();
				});
			}

			// AMQP
			const amqpAuths = ['pub_sub.amqp.auth.user', 'pub_sub.amqp.auth.password'];
			if (this.addAmqpAuthentication) {
				amqpAuths?.forEach((item: string) => {
					this.sourceForm.get(item)?.addValidators(Validators.required);
					this.sourceForm.get(item)?.updateValueAndValidity();
				});
			} else {
				amqpAuths?.forEach((item: string) => {
					this.sourceForm.get(item)?.removeValidators(Validators.required);
					this.sourceForm.get(item)?.updateValueAndValidity();
				});
			}

			const amqpExchange = ['pub_sub.amqp.exchange.routingKey', 'pub_sub.amqp.exchange.exchange'];
			if (this.addAmqpQueueBinding) {
				amqpExchange?.forEach((item: string) => {
					this.sourceForm.get(item)?.addValidators(Validators.required);
					this.sourceForm.get(item)?.updateValueAndValidity();
				});
			} else {
				amqpExchange?.forEach((item: string) => {
					this.sourceForm.get(item)?.removeValidators(Validators.required);
					this.sourceForm.get(item)?.updateValueAndValidity();
				});
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

	setupTransformDialog() {
		document.getElementById(this.showAction === 'true' ? 'subscriptionForm' : 'configureProjectForm')?.scroll({ top: 0, behavior: 'smooth' });
		this.showTransformDialog = true;
	}

	getFunction(functionDetails: { body: any; header: any }) {
		this.sourceForm.get('body_function')?.patchValue(functionDetails.body);
		this.sourceForm.get('header_function')?.patchValue(functionDetails.header);
		this.showTransformDialog = false;
	}
}
