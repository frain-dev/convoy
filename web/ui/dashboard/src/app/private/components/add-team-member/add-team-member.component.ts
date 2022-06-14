import { Location } from '@angular/common';
import { Component, OnInit } from '@angular/core';
import { FormBuilder, FormControl, FormGroup, Validators } from '@angular/forms';
import { Router } from '@angular/router';
import { APP } from 'src/app/models/app.model';
import { GROUP } from 'src/app/models/group.model';
import { GeneralService } from 'src/app/services/general/general.service';
import { AddTeamMemberService } from './add-team-member.service';

@Component({
	selector: 'app-add-team-member',
	templateUrl: './add-team-member.component.html',
	styleUrls: ['./add-team-member.component.scss']
})
export class AddTeamMemberComponent implements OnInit {
	invitingUser = false;
	showSuccessModal = false;
	inviteUserForm: FormGroup = this.formBuilder.group({
		invitee_email: ['', Validators.compose([Validators.required, Validators.email])],
		role: this.formBuilder.group({
			type: ['super_user', Validators.required]
		})
	});

	constructor(private formBuilder: FormBuilder, private generalService: GeneralService, private addTeamService: AddTeamMemberService, private location: Location, private router:Router) {}

	ngOnInit() {
	}

	async inviteUser() {
		if (this.inviteUserForm.invalid) {
			(<any>this.inviteUserForm).values(this.inviteUserForm.controls).forEach((control: FormControl) => {
				control?.markAsTouched();
			});
			return;
		}
		this.invitingUser = true;
		try {
			const response = await this.addTeamService.inviteUserToOrganisation(this.inviteUserForm.value);
			if (response.data) this.showSuccessModal = true;
			this.inviteUserForm.reset();
			this.invitingUser = false;
			this.goBack();
		} catch {
			this.invitingUser = false;
		}
	}


	goBack() {
		this.location.back();
	}
}
