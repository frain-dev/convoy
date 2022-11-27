import { Component, OnInit } from '@angular/core';
import { CommonModule } from '@angular/common';
import { ButtonComponent } from 'src/app/components/button/button.component';
import { PrivateService } from '../../private.service';
import { FormBuilder, FormControl, FormGroup, ReactiveFormsModule, Validators } from '@angular/forms';
import { ModalComponent } from 'src/app/components/modal/modal.component';
import { InputDirective, InputErrorComponent, InputFieldDirective, LabelComponent } from 'src/app/components/input/input.component';
import { GeneralService } from 'src/app/services/general/general.service';
import { Router } from '@angular/router';
import { LoaderModule } from '../../components/loader/loader.module';

export type STAGES = 'organisation' | 'project';

@Component({
	selector: 'convoy-onboarding',
	standalone: true,
	imports: [CommonModule, ButtonComponent, ReactiveFormsModule, ModalComponent, LoaderModule, InputFieldDirective, InputErrorComponent, InputDirective, LabelComponent],
	templateUrl: './onboarding.component.html',
	styleUrls: ['./onboarding.component.scss']
})
export class OnboardingComponent implements OnInit {
	onboardingSteps = [
		{ step: 'Create an Organization', id: 'organisation', description: 'Add your organization details and get set up.', stepColor: 'bg-[#416FF4] shadow-[0_22px_24px_0px_rgba(65,111,244,0.2)]', class: 'border-[rgba(65,111,244,0.2)]', currentStage: 'current' },
		{
			step: 'Create your first project',
			id: 'project',
			description: 'Set up all the information for creating your webhook events.',
			stepColor: 'bg-[#47B38D] shadow-[0_22px_24px_0px_rgba(43,214,123,0.2)]',
			class: 'border-[rgba(71,179,141,0.36)]',
			currentStage: 'pending'
		}
	];
	showCreateOrganisationModal = false;
	creatingOrganisation = false;
	hasProjects: boolean = true;
	isloading = false;
	addOrganisationForm: FormGroup = this.formBuilder.group({
		name: ['', Validators.required]
	});

	constructor(private privateService: PrivateService, private formBuilder: FormBuilder, private generalService: GeneralService, public router: Router) {}

	ngOnInit() {
		this.getOrganizations();
	}

	async addNewOrganisation() {
		if (this.addOrganisationForm.invalid) {
			(<any>this.addOrganisationForm).values(this.addOrganisationForm.controls).forEach((control: FormControl) => {
				control?.markAsTouched();
			});
			return;
		}
		this.creatingOrganisation = true;

		try {
			const response = await this.privateService.addOrganisation(this.addOrganisationForm.value);
			this.generalService.showNotification({ style: 'success', message: response.message });
			this.creatingOrganisation = false;
			location.reload();
			this.showCreateOrganisationModal = false;
		} catch {
			this.creatingOrganisation = false;
		}
	}

	async getOrganizations() {
		this.isloading = true;
		try {
			const response = await this.privateService.getOrganizations();
			const organisations = response.data.content;
			if (organisations?.length) {
				this.updateStep({ currentStep: 'project', prevStep: 'organisation' });
				this.isloading = false;
				return this.getProjects();
			}

			this.showCreateOrganisationModal = true;
			this.isloading = false;
			return;
		} catch (error) {
			this.isloading = false;
			return error;
		}
	}

	async getProjects() {
		this.isloading = true;
		try {
			const projectsResponse = await this.privateService.getProjects();
			const projects = projectsResponse.data;
			this.isloading = false;

			if (projects.length > 0) {
				this.hasProjects = true;
				return this.router.navigateByUrl('/projects');
			}

			this.hasProjects = false;
		} catch (error) {
			this.hasProjects = false;
			this.isloading = false;
			return error;
		}
	}

	updateStep(steps: { currentStep: STAGES; prevStep: STAGES }) {
		this.onboardingSteps.forEach(item => {
			if (item.id === steps.currentStep) item.currentStage = 'current';
			if (item.id === steps.prevStep) item.currentStage = 'done';
		});
	}
}
