import { Component, EventEmitter, Input, OnInit, Output } from '@angular/core';
import { FormGroup, Validators, FormBuilder } from '@angular/forms';
import { Router } from '@angular/router';
import { GROUP } from 'src/app/models/group.model';
import { GeneralService } from 'src/app/services/general/general.service';
import { PrivateService } from '../../private.service';
import { CreateProjectComponentService } from './create-project-component.service';

@Component({
	selector: 'app-create-project-component',
	templateUrl: './create-project-component.component.html',
	styleUrls: ['./create-project-component.component.scss']
})
export class CreateProjectComponent implements OnInit {
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
				hash: [null]
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
	isCreatingProject = false;
	showApiKey = false;
	enableMoreConfig = false;
	apiKey!: string;
	hashAlgorithms = ['SHA256', 'SHA512', 'MD5', 'SHA1', 'SHA224', 'SHA384', 'SHA3_224', 'SHA3_256', 'SHA3_384', 'SHA3_512', 'SHA512_256', 'SHA512_224'];
	retryLogicTypes = [
		{ uid: 'linear', name: 'Linear time retry' },
		{ uid: 'exponential', name: 'Exponential time backoff' }
	];
	@Output('onAction') onAction = new EventEmitter<any>();
	@Input('action') action: 'create' | 'update' = 'create';
	projectDetails!: GROUP;

	constructor(private formBuilder: FormBuilder, private createProjectService: CreateProjectComponentService, private generalService: GeneralService, private privateService: PrivateService, public router: Router) {}

	ngOnInit(): void {
		if (this.action === 'update') this.getProjectDetails();
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
			console.log(this.projectForm.value);
		} catch (error) {
			console.log(error);
		}
	}

	async createProject() {
		if (this.projectForm.invalid) return this.projectForm.markAllAsTouched();

		this.enableMoreConfig ? this.checkProjectConfig() : delete this.projectForm.value.config;
		console.log(this.projectForm.value);

		this.isCreatingProject = true;

		try {
			const response = await this.createProjectService.createProject(this.projectForm.value);
			this.isCreatingProject = false;
			this.projectForm.reset();
			this.privateService.activeProjectDetails = response.data.group;
			this.generalService.showNotification({ message: 'Project created successfully!', style: 'success' });
			this.apiKey = response.data.api_key.key;
			this.projectDetails = response.data.group;
			this.showApiKey = true;
		} catch (error) {
			this.isCreatingProject = false;
		}
	}

	async updateProject() {
		if (this.projectForm.invalid) return this.projectForm.markAllAsTouched();
		this.projectForm.value.config.ratelimit.duration = this.getTimeValue(this.projectForm.value.config.ratelimit.duration);
		this.projectForm.value.config.strategy.duration = this.getTimeValue(this.projectForm.value.config.strategy.duration);
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

	checkProjectConfig() {
		const configDetails = this.projectForm.value.config;
		const configKeys = Object.keys(configDetails).slice(0, -1);
		configKeys.forEach(configKey => {
			const configKeyValues = Object.values(configDetails[configKey]);
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
