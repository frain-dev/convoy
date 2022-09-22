import { Component, OnInit } from '@angular/core';
import { CommonModule } from '@angular/common';
import { ButtonComponent } from 'src/app/components/button/button.component';
import { PrivateService } from '../../private.service';
import { FormBuilder, FormControl, FormGroup, ReactiveFormsModule, Validators } from '@angular/forms';
import { ModalComponent } from 'src/app/components/modal/modal.component';
import { InputComponent } from 'src/app/components/input/input.component';
import { GeneralService } from 'src/app/services/general/general.service';

export type STAGES = 'creatOrganisation' | 'createProject' | 'sendEvent';

@Component({
	selector: 'convoy-onboarding',
	standalone: true,
	imports: [CommonModule, ButtonComponent, ReactiveFormsModule, ModalComponent, InputComponent],
	templateUrl: './onboarding.component.html',
	styleUrls: ['./onboarding.component.scss']
})
export class OnboardingComponent implements OnInit {
	onboardingSteps = [
		{ step: 'Create an Organization', description: 'Add your organization details and get set up.', stepColor: 'bg-[#416FF4] shadow-[0_22px_24px_0px_rgba(65,111,244,0.2)]', border: 'rgba(65,111,244,0.2)' },
		{ step: 'Create your first project', description: 'Add your organization details and get set up.', stepColor: 'bg-[#47B38D] shadow-[0_22px_24px_0px_rgba(43,214,123,0.2)]', border: 'rgba(71,179,141,0.36)' },
		{ step: 'Set up your first webhook event', description: 'Add your organization details and get set up.', stepColor: 'bg-[#F0AD4E] shadow-[0_22px_24px_0px_rgba(247,227,109,0.2)]', border: 'rgba(240,173,78,0.2)' }
	];
	showCreateModal = false;
	creatingOrganisation = false;
	addOrganisationForm: FormGroup = this.formBuilder.group({
		name: ['', Validators.required]
	});

	constructor(private privateService: PrivateService, private formBuilder: FormBuilder, private generalService: GeneralService) {}

	ngOnInit(): void {}

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
			this.showCreateModal = false;
			this.creatingOrganisation = false;
		} catch {
			this.creatingOrganisation = false;
		}
	}
}
