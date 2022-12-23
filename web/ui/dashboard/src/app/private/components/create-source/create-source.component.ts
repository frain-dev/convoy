import { Component, EventEmitter, Input, OnInit, Output } from '@angular/core';
import { FormBuilder, FormGroup, Validators } from '@angular/forms';
import { ActivatedRoute, Router } from '@angular/router';
import { PrivateService } from '../../private.service';
import { CreateSourceService } from './create-source.service';

@Component({
	selector: 'convoy-create-source',
	templateUrl: './create-source.component.html',
	styleUrls: ['./create-source.component.scss']
})
export class CreateSourceComponent implements OnInit {
	@Input('action') action: 'update' | 'create' = 'create';
	@Output() onAction = new EventEmitter<any>();
	sourceForm: FormGroup = this.formBuilder.group({
		name: ['', Validators.required],
		is_disabled: [true, Validators.required],
		type: ['http', Validators.required],
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
	sourceId = this.route.snapshot.params.id;
	isloading = false;
	confirmModal = false;

	constructor(private formBuilder: FormBuilder, private createSourceService: CreateSourceService, private privateService: PrivateService, private route: ActivatedRoute, private router: Router) {}

	ngOnInit(): void {
		this.action === 'update' ? this.getSourceDetails() : this.getSources();
	}

	async getSourceDetails() {
		try {
			const response = await this.createSourceService.getSourceDetails(this.sourceId);
			const sourceProvider = response.data?.provider;
			this.sourceForm.patchValue(response.data);
			if (this.isCustomSource(sourceProvider)) this.sourceForm.patchValue({ verifier: { type: sourceProvider } });

			return;
		} catch (error) {
			return error;
		}
	}

	async saveSource() {
		const verifierType = this.sourceForm.get('verifier.type')?.value;
		const verifier = this.isCustomSource(verifierType) ? 'hmac' : verifierType;

		if (this.sourceForm.get('verifier.type')?.value === 'github') this.sourceForm.get('verifier.hmac')?.patchValue({ encoding: 'hex', header: 'X-Hub-Signature-256', hash: 'SHA256' });
		if (this.sourceForm.get('verifier.type')?.value === 'shopify') this.sourceForm.get('verifier.hmac')?.patchValue({ encoding: 'base64', header: 'X-Shopify-Hmac-SHA256', hash: 'SHA256' });
		if (this.sourceForm.get('verifier.type')?.value === 'twitter') this.sourceForm.get('verifier.hmac')?.patchValue({ encoding: 'base64', header: 'X-Twitter-Webhooks-Signature', hash: 'SHA256' });

		if (!this.isSourceFormValid()) return this.sourceForm.markAllAsTouched();
		const sourceData = {
			...this.sourceForm.value,
			provider: this.isCustomSource(verifierType) ? verifierType : '',
			verifier: {
				type: verifier,
				[verifier]: { ...this.sourceForm.get('verifier.' + verifier)?.value }
			}
		};

		this.isloading = true;
		try {
			const response = this.action === 'update' ? await this.createSourceService.updateSource({ data: sourceData, id: this.sourceId }) : await this.createSourceService.createSource({ sourceData });
			this.isloading = false;
			this.onAction.emit({ action: this.action, data: response.data });
			document.getElementById('configureProjectForm')?.scroll({ top: 0, behavior: 'smooth' });
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

		if (this.sourceForm.get('verifier')?.value.type === 'noop') return true;

		if (this.sourceForm.get('verifier')?.value.type === 'api_key' && this.sourceForm.get('verifier.api_key')?.valid) return true;

		if (this.sourceForm.get('verifier')?.value.type === 'basic_auth' && this.sourceForm.get('verifier.basic_auth')?.valid) return true;

		if ((this.sourceForm.get('verifier')?.value.type === 'hmac' || this.isCustomSource(this.sourceForm.get('verifier.type')?.value)) && this.sourceForm.get('verifier.hmac')?.valid) return true;

		return false;
	}

	cancel() {
		document.getElementById(this.router.url.includes('/configure') ? 'configureProjectForm' : 'sourceForm')?.scroll({ top: 0, behavior: 'smooth' });
		this.confirmModal = true;
	}

	isNewProjectRoute(): boolean {
		if (this.router.url == '/projects/new') return true;
		return false;
	}
}
