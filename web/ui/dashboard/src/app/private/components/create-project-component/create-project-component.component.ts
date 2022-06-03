import { Component, EventEmitter, Input, OnInit, Output } from '@angular/core';
import { FormGroup, Validators, FormBuilder, FormControl } from '@angular/forms';
import { GeneralService } from 'src/app/services/general/general.service';
import { CreateProjectComponentService } from './create-project-component.service';

@Component({
	selector: 'app-create-project-component',
	templateUrl: './create-project-component.component.html',
	styleUrls: ['./create-project-component.component.scss']
})
export class CreateProjectComponent implements OnInit {
	projectForm: FormGroup = this.formBuilder.group({
		name: ['', Validators.required],
		strategy: this.formBuilder.group({
			duration: ['', Validators.required],
			retry_count: ['', Validators.required],
			type: ['', Validators.required]
		}),
		signature: this.formBuilder.group({
			header: ['', Validators.required],
			hash: ['', Validators.required]
		}),
		type: ['', Validators.required],
		rate_limit: ['', Validators.required],
		rate_limit_duration: ['', Validators.required],
		disable_endpoint: [false, Validators.required]
	});
	isCreatingProject = false;
	hashAlgorithms = ['SHA256', 'SHA512', 'MD5', 'SHA1', 'SHA224', 'SHA384', 'SHA3_224', 'SHA3_256', 'SHA3_384', 'SHA3_512', 'SHA512_256', 'SHA512_224'];
	retryLogicTypes = [
		{ id: 'linear', type: 'Linear time retry' },
		{ id: 'exponential', type: 'Exponential time backoff' }
	];
	@Output('onAction') onAction = new EventEmitter<any>();
	@Input('action') action: 'create' | 'update' = 'create';

	constructor(private formBuilder: FormBuilder, private createProjectService: CreateProjectComponentService, private generalService: GeneralService) {}

	ngOnInit(): void {
		if (this.action === 'update') {
		}
	}

	async createProject() {
		if (this.projectForm.invalid) return this.projectForm.markAllAsTouched();

		this.isCreatingProject = true;
		const [digits] = this.projectForm.value.strategy.duration.match(/\D+|\d+/g);
		this.projectForm.value.strategy.duration = parseInt(digits) * 1000;
		try {
			const response = await this.createProjectService.createProject(this.projectForm.value);
			this.generalService.showNotification({ message: 'Project created successfully!', style: 'success' });
			this.onAction.emit({ action: 'createProject', data: response.data });
		} catch (error) {
			this.isCreatingProject = false;
		}
	}
}
