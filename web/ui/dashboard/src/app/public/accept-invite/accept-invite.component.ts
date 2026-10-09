import { Component, OnInit } from '@angular/core';
import { FormBuilder, FormGroup, ReactiveFormsModule, Validators } from '@angular/forms';
import { ActivatedRoute, Router, RouterModule } from '@angular/router';
import { InputErrorComponent } from 'src/app/components/input/input.component';
import { AuthShellComponent } from 'src/app/public/components/auth-shell/auth-shell.component';
import { LoaderModule } from 'src/app/private/components/loader/loader.module';
import { GeneralService } from 'src/app/services/general/general.service';
import { PrivateService } from 'src/app/private/private.service';
import { AcceptInviteService } from './accept-invite.service';
import { inviteAcceptPath, rememberInviteRedirect, sessionHasAccessToken } from './invite-redirect';

@Component({
	selector: 'app-accept-invite',
	imports: [ReactiveFormsModule, LoaderModule, InputErrorComponent, RouterModule, AuthShellComponent],
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
	inviteeEmail = '';
	sessionEmail = '';
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

	constructor(
		private formBuilder: FormBuilder,
		private acceptInviteService: AcceptInviteService,
		private route: ActivatedRoute,
		private router: Router,
		private generalService: GeneralService,
		private privateService: PrivateService
	) {}

	get isInviteeSession(): boolean {
		return sessionHasAccessToken() && this.emailsMatch(this.sessionEmail, this.inviteeEmail);
	}

	ngOnInit() {
		const token = this.route.snapshot.queryParams['invite-token'];
		this.sessionEmail = this.readSessionEmail();
		this.getUserDetails(token);
	}

	async getUserDetails(token: string) {
		this.fetchingDetails = true;
		try {
			const response = await this.acceptInviteService.getUserDetails(token);
			this.userDetailsAvailable = !!response.data.user_exists;

			const inviteeDetails = response.data.token;
			if (inviteeDetails?.organisation_name) this.organisationName = inviteeDetails?.organisation_name;
			inviteeDetails.status === 'accepted' ? (this.isInviteAccepted = true) : (this.isInviteAccepted = false);

			this.inviteeEmail = inviteeDetails.invitee_email || '';
			this.acceptInviteForm.patchValue({
				first_name: response.data.first_name ? response.data.first_name : '',
				last_name: response.data.last_name ? response.data.last_name : '',
				email: this.inviteeEmail,
				role: { type: inviteeDetails.role.type }
			});

			this.fetchingDetails = false;
		} catch {
			this.fetchingDetails = false;
		}
	}

	signInAsInvitee() {
		const token = this.route.snapshot.queryParams['invite-token'];
		if (typeof token !== 'string' || token.length === 0) return;
		const path = rememberInviteRedirect(inviteAcceptPath(token)) || inviteAcceptPath(token);
		localStorage.removeItem('CONVOY_AUTH');
		localStorage.removeItem('CONVOY_AUTH_TOKENS');
		this.router.navigate(['/login'], { queryParams: { redirect: path } });
	}

	async acceptInvite() {
		if (this.userDetailsAvailable && !this.isInviteeSession) {
			this.signInAsInvitee();
			return;
		}
		if (!this.userDetailsAvailable && this.acceptInviteForm.invalid) return this.acceptInviteForm.markAllAsTouched();
		const body = { ...this.acceptInviteForm.value };
		if (this.userDetailsAvailable) {
			delete body.password;
			delete body.first_name;
			delete body.last_name;
		}

		this.loading = true;
		try {
			const token = this.route.snapshot.queryParams['invite-token'];
			const response = await this.acceptInviteService.acceptInvite({ token: token, body });
			this.loading = false;

			this.generalService.showNotification({ style: 'success', message: response.message });

			if (this.userDetailsAvailable) {
				try {
					await this.privateService.getOrganizations({ refresh: true });
				} catch {
					// membership already landed; a stale org list is recoverably wrong
				}
				this.router.navigateByUrl('projects');
			} else {
				this.router.navigateByUrl('login');
			}
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

	private readSessionEmail(): string {
		if (!sessionHasAccessToken()) return '';
		const raw = localStorage.getItem('CONVOY_AUTH');
		if (!raw || raw === 'undefined') return '';
		try {
			const auth = JSON.parse(raw);
			return typeof auth?.email === 'string' ? auth.email : '';
		} catch {
			return '';
		}
	}

	private emailsMatch(a: string, b: string): boolean {
		const left = (a || '').trim();
		const right = (b || '').trim();
		return left.length > 0 && left.toLowerCase() === right.toLowerCase();
	}
}
