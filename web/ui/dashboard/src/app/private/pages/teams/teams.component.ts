import { Component, OnInit } from '@angular/core';
import { FormBuilder, FormControl, FormGroup, Validators } from '@angular/forms';
import { ActivatedRoute, Router } from '@angular/router';
import { PAGINATION } from 'src/app/models/global.model';
import { TEAMS } from 'src/app/models/teams.model';
import { GeneralService } from 'src/app/services/general/general.service';
import { TeamsService } from './teams.service';

@Component({
	selector: 'app-teams',
	templateUrl: './teams.component.html',
	styleUrls: ['./teams.component.scss']
})
export class TeamsComponent implements OnInit {
	tableHead: string[] = ['Name', 'Role', 'Projects', ''];
	filterOptions: ['active', 'pending'] = ['active', 'pending'];
	showInviteTeamMemberModal = this.router.url.split('/')[2]?.includes('new');
	showDeactivateModal = false;
	showCancelInviteModal = false;
	cancelingInvite = false;
	selectedMember!: TEAMS;
	isFetchingTeamMembers = false;
	isFetchingPendingInvites = false;
	deactivatingUser = false;
	searchString!: string;
	organisationId!: string;
	teams!: { pagination: PAGINATION; content: TEAMS[] };
	pendingInvites!: { pagination: PAGINATION; content: TEAMS[] };
	currentId!: string;
	selectedFilterOption: 'active' | 'pending' = 'active';
	showOverlay = false;
	noData = false;
	noInvitesData = false;
	showFilterDropdown = false;
	invitingUser = false;
	showPendingInvitesDropdown = false;
	inviteUserForm: FormGroup = this.formBuilder.group({
		invitee_email: ['', Validators.compose([Validators.required, Validators.email])],
		role: this.formBuilder.group({
			type: ['super_user', Validators.required]
		})
	});

	constructor(private generalService: GeneralService, private router: Router, private route: ActivatedRoute, private teamService: TeamsService, private formBuilder: FormBuilder) {}

	ngOnInit() {
		this.toggleFilter(this.route.snapshot.queryParams?.inviteType ?? 'active');
	}

	async fetchTeamMembers(requestDetails?: { searchString?: string; page?: number }) {
		this.isFetchingTeamMembers = true;
		const page = requestDetails?.page || this.route.snapshot.queryParams.page || 1;
		try {
			const response = await this.teamService.getTeamMembers({ pageNo: page, searchString: requestDetails?.searchString });
			this.teams = response.data;
			response.data.content.length === 0 ? (this.noData = true) : (this.noData = false);

			this.isFetchingTeamMembers = false;
		} catch {
			this.isFetchingTeamMembers = false;
		}
	}

	toggleFilter(selectedFilter: 'active' | 'pending') {
		this.selectedFilterOption = selectedFilter;
		this.selectedFilterOption === 'active' ? this.fetchTeamMembers() : this.fetchPendingTeamMembers();
		if (!this.router.url.split('/')[2]) this.addFilterToUrl();
	}
	async fetchPendingTeamMembers(requestDetails?: { page?: number }) {
		this.isFetchingPendingInvites = true;
		const page = requestDetails?.page || this.route.snapshot.queryParams.pendingInvites || 1;
		try {
			const response = await this.teamService.getPendingTeamMembers({ pageNo: page });
			this.pendingInvites = response.data;
			response.data.content.length === 0 ? (this.noInvitesData = true) : (this.noInvitesData = false);
			this.isFetchingPendingInvites = false;
		} catch {
			this.isFetchingPendingInvites = false;
		}
	}

	searchTeam(searchDetails: { searchInput?: any }) {
		const searchString: string = searchDetails?.searchInput?.target?.value || this.searchString;
		this.fetchTeamMembers({ searchString: searchString });
	}

	async deactivateMember() {
		this.deactivatingUser = true;
		const requestOptions = {
			memberId: this.selectedMember?.uid
		};
		try {
			const response = await this.teamService.deactivateTeamMember(requestOptions);
			if (response.status) this.showDeactivateModal = false;
			this.generalService.showNotification({ style: 'success', message: response.message });
			this.fetchTeamMembers();
			this.deactivatingUser = false;
		} catch {
			this.deactivatingUser = false;
		}
	}

	addFilterToUrl() {
		const queryParams: any = {};
		queryParams.inviteType = this.selectedFilterOption;
		this.router.navigate([], { queryParams: Object.assign({}, queryParams) });
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
			const response = await this.teamService.inviteUserToOrganisation(this.inviteUserForm.value);
			this.generalService.showNotification({ message: response.message, style: 'success' });
			this.inviteUserForm.reset();
			this.invitingUser = false;
			this.router.navigate(['/team'], { queryParams: { inviteType: 'pending' } });
		} catch {
			this.invitingUser = false;
		}
	}

	async resendInvite(inviteId: string) {
		try {
			const response = await this.teamService.resendPendingInvite(inviteId);
			this.generalService.showNotification({ message: response.message, style: 'success' });
			this.fetchPendingTeamMembers();
		} catch {}
	}

	async cancelInvite() {
		this.cancelingInvite = true;
		try {
			const response = await this.teamService.cancelPendingInvite(this.currentId);
			this.generalService.showNotification({ message: response.message, style: 'success' });
			this.fetchPendingTeamMembers();
			this.currentId = '';
			this.showCancelInviteModal = false;
			this.cancelingInvite = false;
		} catch {
			this.cancelingInvite = false;
		}
	}
}
