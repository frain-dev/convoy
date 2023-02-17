import { Component, EventEmitter, Input, OnInit, Output } from '@angular/core';
import { FormBuilder, FormGroup, Validators } from '@angular/forms';
import { ActivatedRoute, Router } from '@angular/router';
import { SOURCE } from 'src/app/models/group.model';
import { GeneralService } from 'src/app/services/general/general.service';
import { PrivateService } from '../../private.service';
import { CreateSourceService } from './create-source.service';

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
		verifier: this.formBuilder.group({
			api_key: this.formBuilder.group({
				header_name: ['', Validators.required],
				header_value: ['', Validators.required]
			}),
			basic_auth: this.formBuilder.group({
				password: ['', Validators.required],
				username: ['', Validators.required]
			}),
			hmac: this.formBuilder.group({
				encoding: ['', Validators.required],
				hash: ['', Validators.required],
				header: ['', Validators.required],
				secret: ['', Validators.required]
			}),
			type: ['', Validators.required]
		})
	});
	sourceTypes = [
		{ value: 'http', viewValue: 'Ingestion HTTP', description: 'Trigger webhook event from a thirdparty webhook event' },
		{ value: 'pub_sub', viewValue: 'Pub/Sub (Coming Soon)', description: 'Trigger webhook event from your Pub/Sub messaging system' },
		{ value: 'db_change_stream', viewValue: 'DB Change Stream (Coming Soon)', description: 'Trigger webhook event from your DB change stream' }
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
	sourceDetails!: SOURCE;
	constructor(private formBuilder: FormBuilder, private createSourceService: CreateSourceService, public privateService: PrivateService, private route: ActivatedRoute, private router: Router, private generalService: GeneralService) {}

	ngOnInit(): void {
		if (this.action === 'update') this.getSourceDetails();
		this.privateService.activeProjectDetails?.type === 'incoming' ? this.sourceForm.patchValue({ type: 'http' }) : this.sourceForm.patchValue({ type: 'pub_sub' });
	}

	async getSourceDetails() {
		try {
			const response = await this.createSourceService.getSourceDetails(this.sourceId);
			this.sourceDetails = response.data;
			const sourceProvider = response.data?.provider;
			this.sourceForm.patchValue(response.data);
			if (this.isCustomSource(sourceProvider)) this.sourceForm.patchValue({ verifier: { type: sourceProvider } });
			return;
		} catch (error) {
			return error;
		}
	}

	checkSourceSetup() {
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
	}

	async saveSource() {
		const sourceData = this.checkSourceSetup();
		if (!this.isSourceFormValid()) return this.sourceForm.markAllAsTouched();
		this.isloading = true;
		try {
			const response = this.action === 'update' ? await this.createSourceService.updateSource({ data: sourceData, id: this.sourceId }) : await this.createSourceService.createSource({ sourceData });
			this.isloading = false;
			this.onAction.emit({ action: this.action, data: response.data });
			document.getElementById('configureProjectForm')?.scroll({ top: 0, behavior: 'smooth' });
			return response;
		} catch (error) {
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

	isSourceFormValid(): boolean {
		if (this.sourceForm.get('name')?.invalid || this.sourceForm.get('type')?.invalid) return false;

		if (this.privateService.activeProjectDetails?.type === 'incoming') {
			if (this.sourceForm.get('verifier')?.value.type === 'noop') return true;

			if (this.sourceForm.get('verifier')?.value.type === 'api_key' && this.sourceForm.get('verifier.api_key')?.valid) return true;

			if (this.sourceForm.get('verifier')?.value.type === 'basic_auth' && this.sourceForm.get('verifier.basic_auth')?.valid) return true;

			if ((this.sourceForm.get('verifier')?.value.type === 'hmac' || this.isCustomSource(this.sourceForm.get('verifier.type')?.value)) && this.sourceForm.get('verifier.hmac')?.valid) return true;
		}

		if (this.privateService.activeProjectDetails?.type === 'outgoing') {
			if (this.sourceForm.get('pub_sub')?.value.type === 'google' && this.sourceForm.get('pub_sub.google')?.valid && this.sourceForm.get('pub_sub.workers')?.valid) return true;

			if (this.sourceForm.get('pub_sub')?.value.type === 'sqs' && this.sourceForm.get('pub_sub.sqs')?.valid && this.sourceForm.get('pub_sub.workers')?.valid) return true;
		}

		return false;
	}

	cancel() {
		document.getElementById(this.router.url.includes('/configure') ? 'configureProjectForm' : 'sourceForm')?.scroll({ top: 0, behavior: 'smooth' });
		this.confirmModal = true;
	}

	setRegionValue(value: any) {
		this.sourceForm.get('pub_sub.sqs')?.patchValue({ default_region: value });
	}
}
