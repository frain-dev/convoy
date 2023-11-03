import { Component, ElementRef, OnInit, ViewChild } from '@angular/core';
import { CommonModule } from '@angular/common';
import { PrivateService } from '../../private.service';
import { Router } from '@angular/router';
import { LoaderModule } from '../../components/loader/loader.module';
import { ButtonComponent } from 'src/app/components/button/button.component';
import { FormBuilder, FormControl, FormGroup, ReactiveFormsModule, Validators } from '@angular/forms';
import { GeneralService } from 'src/app/services/general/general.service';
import { InputFieldDirective, InputErrorComponent, InputDirective, LabelComponent } from 'src/app/components/input/input.component';
import { DialogDirective } from 'src/app/components/dialog/dialog.directive';

export type STAGES = 'organisation' | 'project';

@Component({
	selector: 'convoy-onboarding',
	standalone: true,
	templateUrl: './onboarding.component.html',
	styleUrls: ['./onboarding.component.scss'],
	imports: [CommonModule, LoaderModule, ButtonComponent, ReactiveFormsModule, InputFieldDirective, InputDirective, LabelComponent, InputErrorComponent, DialogDirective]
})
export class OnboardingComponent implements OnInit {
	@ViewChild('orgDialog', { static: true }) dialog!: ElementRef<HTMLDialogElement>;
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
	hasProjects: boolean = true;
	isloading = false;
	addOrganisationForm: FormGroup = this.formBuilder.group({
		name: ['', Validators.required]
	});
	creatingOrganisation = false;

	constructor(public privateService: PrivateService, public router: Router, private generalService: GeneralService, private formBuilder: FormBuilder) {}

	ngOnInit() {
		this.getOrganizations();
	}

	async getOrganizations(refresh: boolean = false) {
		this.isloading = true;

		try {
			const response = await this.privateService.getOrganizations({ refresh });
			const organisations = response.data.content;

			if (organisations?.length) {
				this.updateStep({ currentStep: 'project', prevStep: 'organisation' });
				this.isloading = false;
			}

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
			this.dialog.nativeElement.close();

			await this.getOrganizations(true);
		} catch {
			this.creatingOrganisation = false;
		}
	}
}
