import { CommonModule } from '@angular/common';
import { Component, OnInit } from '@angular/core';
import { FormBuilder, FormControl, FormGroup, ReactiveFormsModule, Validators } from '@angular/forms';
import { ActivatedRoute, Router } from '@angular/router';
import { ButtonComponent } from 'src/app/components/button/button.component';
import { InputDirective, InputErrorComponent, InputFieldDirective, LabelComponent } from 'src/app/components/input/input.component';
import { LoaderModule } from 'src/app/private/components/loader/loader.module';
import { GeneralService } from 'src/app/services/general/general.service';
import { AcceptInviteService } from './accept-invite.service';

@Component({
	selector: 'app-accept-invite',
	standalone: true,
	imports: [CommonModule, ReactiveFormsModule, ButtonComponent, LoaderModule, InputFieldDirective, InputErrorComponent, InputDirective, LabelComponent],
	templateUrl: './accept-invite.component.html',
	styleUrls: ['./accept-invite.component.scss']
})
export class AcceptInviteComponent implements OnInit {
	showPassword = false;
	showConfirmPassword = false;
	loading = false;
	fetchingDetails = false;
	userDetailsAvailable = false;
	isInviteAccepted = false;
	acceptInviteForm: FormGroup = this.formBuilder.group({
		first_name: ['', Validators.required],
		last_name: ['', Validators.required],
		email: ['', Validators.required],
		role: this.formBuilder.group({
			type: ['organisation_admin']
		}),
		password: ['', Validators.compose([Validators.minLength(8), Validators.required])]
	});
	organisationName!: string;

	constructor(private formBuilder: FormBuilder, private acceptInviteService: AcceptInviteService, private route: ActivatedRoute, private router: Router, private generalService: GeneralService) {}

	ngOnInit() {
		const token = this.route.snapshot.queryParams['invite-token'];
		this.getUserDetails(token);
	}

	async getUserDetails(token: string) {
		this.fetchingDetails = true;
		try {
			const response = await this.acceptInviteService.getUserDetails(token);
			response.data.user ? (this.userDetailsAvailable = true) : (this.userDetailsAvailable = false);

			const inviteeDetails = response.data.token;
			if (inviteeDetails?.organisation_name) this.organisationName = inviteeDetails?.organisation_name;
			inviteeDetails.status === 'accepted' ? (this.isInviteAccepted = true) : (this.isInviteAccepted = false);
			const userDetails = response.data.user;

			this.acceptInviteForm.patchValue({
				first_name: userDetails?.first_name ? userDetails.first_name : '',
				last_name: userDetails?.last_name ? userDetails.last_name : '',
				email: inviteeDetails.invitee_email,
				role: { type: inviteeDetails.role.type }
			});

			this.fetchingDetails = false;
		} catch {
			this.fetchingDetails = false;
		}
	}
	async acceptInvite() {
		if (!this.userDetailsAvailable && this.acceptInviteForm.invalid) return this.acceptInviteForm.markAllAsTouched();
		if (this.userDetailsAvailable) {
			delete this.acceptInviteForm.value.password;
		}

		this.loading = true;
		try {
			const token = this.route.snapshot.queryParams['invite-token'];
			const response = await this.acceptInviteService.acceptInvite({ token: token, body: this.acceptInviteForm.value });
			this.loading = false;

			const authDetails = localStorage.getItem('CONVOY_AUTH');
			this.generalService.showNotification({ style: 'success', message: response.message });

			this.userDetailsAvailable && authDetails !== 'undefined' ? this.router.navigateByUrl('projects') : this.router.navigateByUrl('login');
		} catch (error: any) {
			this.loading = false;
			this.generalService.showNotification({ style: 'error', message: error.error.message });
		}
	}

	checkForNumber(password: string): boolean {
		const regex = /\d/;
		return regex.test(password);
	}

	checkForSpecialCharacter(password: string): boolean {
		const regex = /[!@#$%^&*()_+\-=\[\]{};':"\\|,.<>\/?]+/;
		return regex.test(password);
	}
}
