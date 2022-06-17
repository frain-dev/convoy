import { Component, EventEmitter, OnInit, Output } from '@angular/core';
import { FormBuilder, FormControl, FormGroup, Validators } from '@angular/forms';
import { GeneralService } from 'src/app/services/general/general.service';
import { CreateOrganisationService } from './create-organisation.service';

@Component({
	selector: 'app-create-organisation',
	templateUrl: './create-organisation.component.html',
	styleUrls: ['./create-organisation.component.scss']
})
export class CreateOrganisationComponent implements OnInit {
	@Output() closeModal = new EventEmitter<any>();
	loading: boolean = false;
	addOrganisationForm: FormGroup = this.formBuilder.group({
		name: ['', Validators.required]
	});
	constructor(private createOrganisationService: CreateOrganisationService, private formBuilder: FormBuilder, private generalService: GeneralService) {}

	ngOnInit(): void {}

	close() {
		this.closeModal.emit();
	}

	async addNewOrganisation() {
		if (this.addOrganisationForm.invalid) {
			(<any>this.addOrganisationForm).values(this.addOrganisationForm.controls).forEach((control: FormControl) => {
				control?.markAsTouched();
			});
			return;
		}
		this.loading = true;
		try {
			const response = await this.createOrganisationService.addOrganisation(this.addOrganisationForm.value);
			if (response.status == true) {
				this.generalService.showNotification({ style: 'success', message: response.message });
				this.closeModal.emit({ action: 'created' });
			}
			this.loading = false;
		} catch {
			this.loading = false;
		}
	}
}
