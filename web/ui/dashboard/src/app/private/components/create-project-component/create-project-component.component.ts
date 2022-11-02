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
				policy: [null]
			}),
			disable_endpoint: [null],
			is_retention_policy_enabled: [null]
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
	showNewSignatureModal = false;
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
			this.projectForm.get('config.ratelimit.duration')?.patchValue(this.getTimeString(response.data.config.ratelimit.duration));
			this.projectForm.get('config.strategy.duration')?.patchValue(this.getTimeString(response.data.config.strategy.duration));

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
		if (this.enableMoreConfig) {
			if (this.newSignatureForm.invalid) return this.newSignatureForm.markAllAsTouched();
			this.versions.at(0).patchValue(this.newSignatureForm.value);
			this.checkProjectConfig();
		}

		if (this.enableMoreConfig && this.projectForm.invalid) return this.projectForm.markAllAsTouched();

		if (!this.enableMoreConfig) delete this.projectForm.value.config;

		this.isCreatingProject = true;

		try {
			const response = await this.createProjectService.createProject(this.projectForm.value);
			window.scrollTo(0, 0);
			this.enableMoreConfig = false;
			this.isCreatingProject = false;
			this.projectForm.reset();
			this.generalService.showNotification({ message: 'Project created successfully!', style: 'success', type: this.privateService.activeProjectDetails?.uid ? 'modal' : 'alert' });
			this.privateService.activeProjectDetails = response.data.group;
			this.apiKey = response.data.api_key.key;
			this.projectDetails = response.data.group;
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

			if (configKey === 'strategy' && configDetails?.strategy?.retry_count) {
				this.projectForm.value.config.strategy.retry_count = parseInt(this.projectForm.value.config.strategy.retry_count);
			}

			if (configKey === 'ratelimit' && configDetails?.ratelimit?.count) {
				this.projectForm.value.config.ratelimit.count = parseInt(this.projectForm.value.config.ratelimit.count);
			}

			if (configKey === 'ratelimit' && configDetails?.ratelimit?.duration) {
				this.projectForm.value.config.ratelimit.duration = this.getTimeValue(configDetails.ratelimit.duration);
			}

			if (configKey === 'strategy' && configDetails?.strategy?.duration) {
				this.projectForm.value.config.strategy.duration = this.getTimeValue(configDetails.strategy.duration);
			}
		});

		if (this.projectForm.value.config.disable_endpoint === null) delete this.projectForm.value.config.disable_endpoint;
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
}
