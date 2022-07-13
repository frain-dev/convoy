import { Component, EventEmitter, Input, OnInit, Output } from '@angular/core';
import { FormBuilder, FormGroup, Validators } from '@angular/forms';
import { ActivatedRoute } from '@angular/router';
import { CreateSourceService } from './create-source.service';

@Component({
	selector: 'app-create-source',
	templateUrl: './create-source.component.html',
	styleUrls: ['./create-source.component.scss']
})
export class CreateSourceComponent implements OnInit {
	@Input('action') action: 'update' | 'create' = 'create';
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
		{ value: 'hmac', viewValue: 'HMAC' },
		{ value: 'basic_auth', viewValue: 'Basic Auth' },
		{ value: 'api_key', viewValue: 'API Key' },
		{ value: 'github', viewValue: 'Github' }
	];
	encodings = ['base64', 'hex'];
	hashAlgorithms = ['SHA256', 'SHA512', 'MD5', 'SHA1', 'SHA224', 'SHA384', 'SHA3_224', 'SHA3_256', 'SHA3_384', 'SHA3_512', 'SHA512_256', 'SHA512_224'];
	sourceId = this.route.snapshot.params.id;
	isloading = false;

	constructor(private formBuilder: FormBuilder, private createSourceService: CreateSourceService, private route: ActivatedRoute) {}

	ngOnInit(): void {
		this.action === 'update' && this.getSourceDetails();
	}

	async getSourceDetails() {
		try {
			const response = await this.createSourceService.getSourceDetails(this.sourceId);
			const sourceProvider = response.data?.provider;
			this.sourceForm.patchValue(response.data);
			if (sourceProvider === 'github') this.sourceForm.patchValue({ verifier: { type: 'github' } });
			return;
		} catch (error) {
			return error;
		}
	}

	async saveSource() {
		const verifier = this.sourceForm.get('verifier.type')?.value === 'github' ? 'hmac' : this.sourceForm.get('verifier.type')?.value;

		if (this.sourceForm.get('verifier.type')?.value === 'github') this.sourceForm.get('verifier.hmac')?.patchValue({ encoding: 'hex', header: 'X-Hub-Signature-256', hash: 'SHA256' });
		if (!this.isSourceFormValid()) return this.sourceForm.markAllAsTouched();

		const sourceData = {
			...this.sourceForm.value,
			provider: this.sourceForm.get('verifier.type')?.value === 'github' ? 'github' : '',
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
		} catch (error) {
			this.isloading = false;
		}
	}

	isSourceFormValid(): boolean {
		if (this.sourceForm.get('name')?.invalid || this.sourceForm.get('type')?.invalid) return false;

		if (this.sourceForm.get('verifier')?.value.type === 'api_key' && this.sourceForm.get('verifier.api_key')?.valid) {
			return true;
		}

		if (this.sourceForm.get('verifier')?.value.type === 'basic_auth' && this.sourceForm.get('verifier.basic_auth')?.valid) {
			return true;
		}

		if ((this.sourceForm.get('verifier')?.value.type === 'hmac' || this.sourceForm.get('verifier')?.value.type === 'github') && this.sourceForm.get('verifier.hmac')?.valid) {
			return true;
		}

		return false;
	}
}
