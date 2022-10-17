import { Component, OnInit } from '@angular/core';
import { CommonModule } from '@angular/common';
import { ButtonComponent } from 'src/app/components/button/button.component';
import { PrivateService } from '../../private.service';
import { FormBuilder, FormControl, FormGroup, ReactiveFormsModule, Validators } from '@angular/forms';
import { ModalComponent } from 'src/app/components/modal/modal.component';
import { InputComponent } from 'src/app/components/input/input.component';
import { GeneralService } from 'src/app/services/general/general.service';
import { Router } from '@angular/router';
import { ORGANIZATION_DATA } from 'src/app/models/organisation.model';
import { LoaderModule } from '../../components/loader/loader.module';

export type STAGES = 'organisation' | 'project';

@Component({
	selector: 'convoy-onboarding',
	standalone: true,
	imports: [CommonModule, ButtonComponent, ReactiveFormsModule, ModalComponent, InputComponent, LoaderModule],
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
	showCreateModal = false;
	creatingOrganisation = false;
	isOrgCreated = false;
	organisations!: ORGANIZATION_DATA[];
	isloadingOrganisations = false;
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
			this.generalService.showNotification({ style: 'success', message: response.message, type: 'modal' });
			this.creatingOrganisation = false;
			location.reload();
			this.updateStep({ currentStep: 'project', prevStep: 'organisation' });
			this.showCreateModal = false;
		} catch {
			this.creatingOrganisation = false;
		}
	}

	async getOrganizations() {
		this.isloadingOrganisations = true;
		try {
			const response = await this.privateService.getOrganizations();
			this.organisations = response.data.content;
			if (this.organisations?.length) {
				this.updateStep({ currentStep: 'project', prevStep: 'organisation' });
				this.getProjects();
			}
			this.isloadingOrganisations = false;
		} catch (error) {
			this.isloadingOrganisations = false;
			return error;
		}
	}

	async getProjects() {
		try {
			const projectsResponse = await this.privateService.getProjects();
			const projects = projectsResponse.data;
			if (projects.length > 0) this.router.navigateByUrl('/projects');
		} catch (error) {
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
