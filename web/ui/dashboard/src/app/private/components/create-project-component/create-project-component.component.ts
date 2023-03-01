import { Component, EventEmitter, Input, OnInit, Output } from '@angular/core';
import { FormGroup, Validators, FormBuilder, FormArray } from '@angular/forms';
import { Router } from '@angular/router';
import { GROUP, VERSIONS } from 'src/app/models/group.model';
import { GeneralService } from 'src/app/services/general/general.service';
import { PrivateService } from '../../private.service';
import { CreateProjectComponentService } from './create-project-component.service';

@Component({
	selector: 'app-create-project-component',
	templateUrl: './create-project-component.component.html',
	styleUrls: ['./create-project-component.component.scss']
})
export class CreateProjectComponent implements OnInit {
	signatureTableHead: string[] = ['Header', 'Version', 'Hash', 'Encoding'];
	projectForm: FormGroup = this.formBuilder.group({
		name: ['', Validators.required],
		config: this.formBuilder.group({
			strategy: this.formBuilder.group({
				duration: [null],
				retry_count: [null],
				type: [null]
			}),
			signature: this.formBuilder.group({
				header: [null],
				versions: this.formBuilder.array([])
			}),
			ratelimit: this.formBuilder.group({
				count: [null],
				duration: [null]
			}),
			retention_policy: this.formBuilder.group({
				policy: ['30d']
			}),
			is_retention_policy_enabled: [true]
		}),
		type: [null, Validators.required]
	});
	newSignatureForm: FormGroup = this.formBuilder.group({
		encoding: [null],
		hash: [null]
	});
	isCreatingProject = false;
	showApiKey = false;
	enableMoreConfig = false;
	confirmModal = false;
	confirmRegenerateKey = false;
	showNewSignatureModal = false;
	regeneratingKey = false;
	apiKey!: string;
	hashAlgorithms = ['SHA256', 'SHA512'];
	retryLogicTypes = [
		{ uid: 'linear', name: 'Linear time retry' },
		{ uid: 'exponential', name: 'Exponential time backoff' }
	];
	encodings = ['base64', 'hex'];
	@Output('onAction') onAction = new EventEmitter<any>();
	@Input('action') action: 'create' | 'update' = 'create';
	projectDetails?: GROUP;
	signatureVersions!: { date: string; content: VERSIONS[] }[];
	configurations = [
		{ uid: 'retry-config', name: 'Retry Config', show: false },
		{ uid: 'rate-limit', name: 'Rate Limit', show: false },
		{ uid: 'retention', name: 'Retention Policy', show: false },
		{ uid: 'signature', name: 'Signature Format', show: false }
	];

	constructor(private formBuilder: FormBuilder, private createProjectService: CreateProjectComponentService, private generalService: GeneralService, private privateService: PrivateService, public router: Router) {}

	ngOnInit(): void {
		if (this.action === 'update') this.getProjectDetails();
	}

	get versions(): FormArray {
		return this.projectForm.get('config.signature.versions') as FormArray;
	}

	get versionsLength(): any {
		const versionsControl = this.projectForm.get('config.signature.versions') as FormArray;
		return versionsControl.length;
	}
	newVersion(): FormGroup {
		return this.formBuilder.group({
			encoding: ['', Validators.required],
			hash: ['', Validators.required]
		});
	}

	addVersion() {
		this.versions.push(this.newVersion());
	}

	toggleMoreConfig(event: any) {
		this.enableMoreConfig = !this.enableMoreConfig;

		if (this.action === 'create') {
			event.target.checked ? this.addVersion() : this.projectForm.get('config.signature.versions')?.reset();
		}

		const strategyControls = Object.keys((this.projectForm.get('config.strategy') as FormGroup).controls);
		const signatureControls = Object.keys((this.projectForm.get('config.signature') as FormGroup).controls);
		const ratelimitControls = Object.keys((this.projectForm.get('config.ratelimit') as FormGroup).controls);
		const retentionPolicyControls = Object.keys((this.projectForm.get('config.retention_policy') as FormGroup).controls);

		if (this.enableMoreConfig) {
			strategyControls.forEach(key => this.projectForm.get(`config.strategy.${key}`)?.setValidators(Validators.required));
			strategyControls.forEach(key => this.projectForm.get(`config.strategy.${key}`)?.updateValueAndValidity());

			signatureControls.forEach(key => this.projectForm.get(`config.signature.${key}`)?.setValidators(Validators.required));
			signatureControls.forEach(key => this.projectForm.get(`config.signature.${key}`)?.updateValueAndValidity());

			ratelimitControls.forEach(key => this.projectForm.get(`config.ratelimit.${key}`)?.setValidators(Validators.required));
			ratelimitControls.forEach(key => this.projectForm.get(`config.ratelimit.${key}`)?.updateValueAndValidity());

			retentionPolicyControls.forEach(key => this.projectForm.get(`config.retention_policy.${key}`)?.setValidators(Validators.required));
			retentionPolicyControls.forEach(key => this.projectForm.get(`config.retention_policy.${key}`)?.updateValueAndValidity());
		} else {
			strategyControls.forEach(key => this.projectForm.get(`config.strategy.${key}`)?.removeValidators(Validators.required));
			strategyControls.forEach(key => this.projectForm.get(`config.strategy.${key}`)?.updateValueAndValidity());

			signatureControls.forEach(key => this.projectForm.get(`config.signature.${key}`)?.removeValidators(Validators.required));
			signatureControls.forEach(key => this.projectForm.get(`config.signature.${key}`)?.updateValueAndValidity());

			ratelimitControls.forEach(key => this.projectForm.get(`config.ratelimit.${key}`)?.removeValidators(Validators.required));
			ratelimitControls.forEach(key => this.projectForm.get(`config.ratelimit.${key}`)?.updateValueAndValidity());

			retentionPolicyControls.forEach(key => this.projectForm.get(`config.retention_policy.${key}`)?.removeValidators(Validators.required));
			retentionPolicyControls.forEach(key => this.projectForm.get(`config.retention_policy.${key}`)?.updateValueAndValidity());
		}
	}

	toggleConfigForm(configValue: string) {
		this.configurations.forEach(config => {
			if (config.uid === configValue) config.show = !config.show;
		});
	}

	showConfig(configValue: string): boolean {
		return this.configurations.find(config => config.uid === configValue)?.show || false;
	}

	async getProjectDetails() {
		this.enableMoreConfig = true;
		try {
			const response = await this.privateService.getProjectDetails();
			this.projectDetails = response.data;

			this.projectForm.patchValue(response.data);
			this.projectForm.get('config.strategy')?.patchValue(response.data.config.strategy);
			this.projectForm.get('config.signature')?.patchValue(response.data.config.signature);
			this.projectForm.get('config.ratelimit')?.patchValue(response.data.config.ratelimit);
			this.configurations.forEach(config => {
				if (this.privateService.activeProjectDetails?.type === 'outgoing') this.toggleConfigForm(config.uid);
				else if (config.uid !== 'signature') this.toggleConfigForm(config.uid);
			});
			const versions = response.data.config.signature.versions;
			if (!versions?.length) return;
			this.signatureVersions = this.generalService.setContentDisplayed(versions);
			versions.forEach((version: { encoding: any; hash: any }, index: number) => {
				this.addVersion();
				this.versions.at(index)?.patchValue({
					encoding: version.encoding,
					hash: version.hash
				});
			});
		} catch (error) {
			console.log(error);
		}
	}

	async createProject() {
		const projectFormModal = document.getElementById('projectForm');

		if (this.enableMoreConfig) {
			if (this.newSignatureForm.invalid || this.projectForm.invalid) {
				this.newSignatureForm.markAllAsTouched();
				this.projectForm.markAllAsTouched();
				projectFormModal?.scroll({ top: 0 });
				return;
			}

			this.versions.at(0).patchValue(this.newSignatureForm.value);
			this.checkProjectConfig();
		}

		if (!this.enableMoreConfig && this.projectForm.get('name')?.invalid && this.projectForm.get('type')?.invalid) {
			projectFormModal?.scroll({ top: 0 });
			return this.projectForm.markAllAsTouched();
		}
		const dataForNoConfig = this.projectForm.value;
		if (!this.enableMoreConfig) delete dataForNoConfig.config;

		this.isCreatingProject = true;

		try {
			const response = await this.createProjectService.createProject(this.enableMoreConfig ? this.projectForm.value : dataForNoConfig);
			projectFormModal?.scroll({ top: 0, behavior: 'smooth' });
			this.isCreatingProject = false;
			this.projectForm.reset();
			this.privateService.activeProjectDetails = response.data.project;
			this.privateService.getProjects();
			this.apiKey = response.data.api_key.key;
			this.projectDetails = response.data.project;
			if (projectFormModal) projectFormModal.style.overflowY = 'hidden';
			this.showApiKey = true;
		} catch (error) {
			this.isCreatingProject = false;
		}
	}

	async updateProject() {
		if (this.projectForm.invalid) return this.projectForm.markAllAsTouched();
		if (typeof this.projectForm.value.config.ratelimit.duration === 'string') this.projectForm.value.config.ratelimit.duration = this.getTimeValue(this.projectForm.value.config.ratelimit.duration);
		if (typeof this.projectForm.value.config.strategy.duration === 'string') this.projectForm.value.config.strategy.duration = this.getTimeValue(this.projectForm.value.config.strategy.duration);
		if (typeof this.projectForm.value.config.strategy.retry_count === 'string') this.projectForm.value.config.strategy.retry_count = parseInt(this.projectForm.value.config.strategy.retry_count);
		if (typeof this.projectForm.value.config.ratelimit.count === 'string') this.projectForm.value.config.ratelimit.count = parseInt(this.projectForm.value.config.ratelimit.count);
		this.isCreatingProject = true;

		try {
			const response = await this.createProjectService.updateProject(this.projectForm.value);
			this.generalService.showNotification({ message: 'Project updated successfully!', style: 'success' });
			this.onAction.emit(response.data);
			this.isCreatingProject = false;
		} catch (error) {
			this.isCreatingProject = false;
		}
	}

	async regenerateKey() {
		this.confirmRegenerateKey = false;
		this.regeneratingKey = true;
		try {
			const response = await this.createProjectService.regenerateKey();
			this.generalService.showNotification({ message: response.message, style: 'success' });
			this.regeneratingKey = false;
			this.apiKey = response.data.key;
			this.showApiKey = true;
		} catch (error) {
			this.regeneratingKey = false;
			return error;
		}
	}

	async createNewSignature(i: number) {
		if (this.newSignatureForm.invalid) return this.newSignatureForm.markAllAsTouched();

		this.versions.at(i).patchValue(this.newSignatureForm.value);
		await this.updateProject();
		this.getProjectDetails();
		this.newSignatureForm.reset();
		this.showNewSignatureModal = false;
	}

	checkProjectConfig() {
		const configDetails = this.projectForm.value.config;
		const configKeys = Object.keys(configDetails).slice(0, -1);
		configKeys.forEach(configKey => {
			const configKeyValues = configDetails[configKey] ? Object.values(configDetails[configKey]) : [];
			if (configKeyValues.every(item => item === null)) delete this.projectForm.value.config[configKey];
		});

		if (this.projectForm.value.config.is_retention_policy_enabled === null) delete this.projectForm.value.config.is_retention_policy_enabled;
	}

	getTimeString(timeValue: number) {
		if (timeValue > 59) return `${Math.round(timeValue / 60)}m`;
		return `${timeValue}s`;
	}

	getTimeValue(timeValue: any) {
		const [digits, word] = timeValue.match(/\D+|\d+/g);
		if (word === 's') return parseInt(digits);
		else if (word === 'm') return parseInt(digits) * 60;
		return parseInt(digits);
	}

	cancel() {
		this.confirmModal = true;
		document.getElementById('projectForm')?.scroll({ top: 0, behavior: 'smooth' });
	}
}
