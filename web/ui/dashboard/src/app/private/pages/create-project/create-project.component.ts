import { Component, OnInit } from '@angular/core';
import { FormBuilder, FormControl, FormGroup, Validators } from '@angular/forms';
import { Router } from '@angular/router';
import { GeneralService } from 'src/app/services/general/general.service';
import { PrivateService } from '../../private.service';
import { CreateProjectService } from './create-project.service';

@Component({
	selector: 'app-create-project',
	templateUrl: './create-project.component.html',
	styleUrls: ['./create-project.component.scss']
})
export class CreateProjectComponent implements OnInit {
	projectStage: 'createProject' | 'createSource' | 'createApplication' | 'createSubscription' = 'createProject';
	hashAlgorithms = ['SHA256', 'SHA512', 'MD5', 'SHA1', 'SHA224', 'SHA384', 'SHA3_224', 'SHA3_256', 'SHA3_384', 'SHA3_512', 'SHA512_256', 'SHA512_224'];
	retryLogicTypes = [
		{ id: 'linear', type: 'Linear time retry' },
		{ id: 'exponential', type: 'Exponential time backoff' }
	];
	isCreatingProject = false;
	projectType: 'incoming' | 'outgoing' = 'outgoing';
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
			rateLimit: this.formBuilder.group({
				count: ['', Validators.required],
				duration: ['', Validators.required]
			})
		}),
		type: ['', Validators.required],
		disable_endpoint: [false, Validators.required]
	});
	constructor(private formBuilder: FormBuilder, private createProjectService: CreateProjectService, private generalService: GeneralService, private privateService: PrivateService, private router: Router) {}

	ngOnInit(): void {}

	async createProject() {
		if (this.projectForm.invalid) {
			(<any>Object).values(this.projectForm.controls).forEach((control: FormControl) => {
				control?.markAsTouched();
			});
			return;
		}
		const projectType = this.projectForm.value.type;
		this.isCreatingProject = true;
		const [digits, word] = this.projectForm.value.config.strategy.duration.match(/\D+|\d+/g);
		word === 's' ? (this.projectForm.value.config.strategy.duration = parseInt(digits) * 1000) : (this.projectForm.value.config.strategy.duration = parseInt(digits) * 1000000);
		try {
			const response = await this.createProjectService.createProject(this.projectForm.value);
			const projectId = response?.data?.uid;
			this.privateService.activeProjectId = projectId;
			this.isCreatingProject = false;
			projectType === 'incoming' ? (this.projectStage = 'createSource') : (this.projectStage = 'createApplication');
			this.generalService.showNotification({ message: 'Project created successfully!', style: 'success' });
		} catch (error) {
			this.isCreatingProject = false;
		}
	}

	toggleActiveStage() {}

	cancel() {
		this.router.navigate(['/projects']);
	}
}
