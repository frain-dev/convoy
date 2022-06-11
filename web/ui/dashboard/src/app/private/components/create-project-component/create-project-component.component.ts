import { Component, EventEmitter, Input, OnInit, Output } from '@angular/core';
import { FormGroup, Validators, FormBuilder } from '@angular/forms';
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
				duration: ['', Validators.required],
				retry_count: ['', Validators.required],
				type: ['', Validators.required]
			}),
			signature: this.formBuilder.group({
				header: ['', Validators.required],
				hash: ['', Validators.required]
			}),
			ratelimit: this.formBuilder.group({
				count: [null, Validators.required],
				duration: [null, Validators.required]
			}),
			disable_endpoint: [false, Validators.required]
		}),
		type: ['', Validators.required]
	});
	isCreatingProject = false;
	hashAlgorithms = ['SHA256', 'SHA512', 'MD5', 'SHA1', 'SHA224', 'SHA384', 'SHA3_224', 'SHA3_256', 'SHA3_384', 'SHA3_512', 'SHA512_256', 'SHA512_224'];
	retryLogicTypes = [
		{ id: 'linear', type: 'Linear time retry' },
		{ id: 'exponential', type: 'Exponential time backoff' }
	];
	@Output('onAction') onAction = new EventEmitter<any>();
	@Input('action') action: 'create' | 'update' = 'create';
	projectDetails!: GROUP;

	constructor(private formBuilder: FormBuilder, private createProjectService: CreateProjectComponentService, private generalService: GeneralService, private privateService: PrivateService) {}

	ngOnInit(): void {
		if (this.action === 'update') this.getProjectDetails();
	}

	async getProjectDetails() {
		try {
			const response = await this.privateService.getProjectDetails();
			this.projectDetails = response.data;
			this.projectForm.patchValue(response.data);
			this.projectForm.get('config.strategy')?.patchValue(response.data.config.strategy);
			this.projectForm.get('config.signature')?.patchValue(response.data.config.signature);
			this.projectForm.get('config.ratelimit')?.patchValue(response.data.config.ratelimit);
		} catch (error) {
			console.log(error);
		}
	}

	async createProject() {
		if (this.projectForm.invalid) return this.projectForm.markAllAsTouched();

		this.isCreatingProject = true;
		const [digits, word] = this.projectForm.value.config.strategy.duration.match(/\D+|\d+/g);
		word === 's' ? (this.projectForm.value.config.strategy.duration = parseInt(digits) * 1000) : (this.projectForm.value.config.strategy.duration = parseInt(digits) * 1000000);

		try {
			const response = await this.createProjectService.createProject(this.projectForm.value);
			this.privateService.activeProjectDetails = response.data;
			this.isCreatingProject = false;
			this.generalService.showNotification({ message: 'Project created successfully!', style: 'success' });
			this.onAction.emit(response.data);
		} catch (error: any) {
			this.isCreatingProject = false;
			this.generalService.showNotification({ message: error.message, style: 'error' });
		}
	}

	async updateProject() {
		if (this.projectForm.invalid) return this.projectForm.markAllAsTouched();

		this.isCreatingProject = true;

		//  Please review, throwing an error
		// const [digits, word] = this.projectForm.value.config.strategy.duration.match(/\D+|\d+/g);
		// word === 's' ? (this.projectForm.value.config.strategy.duration = parseInt(digits) * 1000) : (this.projectForm.value.config.strategy.duration = parseInt(digits) * 1000000);
		try {
			const response = await this.createProjectService.updateProject(this.projectForm.value);
			this.generalService.showNotification({ message: 'Project updated successfully!', style: 'success' });
			this.onAction.emit(response.data);
			this.isCreatingProject = false;
		} catch (error) {
			this.isCreatingProject = false;
		}
	}
}
