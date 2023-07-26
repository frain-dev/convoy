import { Component, OnInit, ViewChild, inject, ElementRef } from '@angular/core';
import { FormBuilder, FormGroup, Validators } from '@angular/forms';
import { ActivatedRoute, Router } from '@angular/router';
import { PAGINATION } from 'src/app/models/global.model';
import { GeneralService } from 'src/app/services/general/general.service';
import { DropdownComponent } from 'src/app/components/dropdown/dropdown.component';
import { TeamsService } from './teams.service';
import { RbacService } from 'src/app/services/rbac/rbac.service';
import { PrivateService } from '../../private.service';
import { TEAM } from 'src/app/models/organisation.model';

@Component({
	selector: 'app-teams',
	templateUrl: './teams.component.html',
	styleUrls: ['./teams.component.scss']
})
export class TeamsComponent implements OnInit {
	@ViewChild(DropdownComponent) dropdownComponent!: DropdownComponent;
	@ViewChild('teamsDialog', { static: true }) teamsDialog!: ElementRef<HTMLDialogElement>;
	@ViewChild('deleteDialog', { static: true }) deleteDialog!: ElementRef<HTMLDialogElement>;

	tableHead: string[] = ['Name', 'Role', 'Projects', ''];
	filterOptions: ['active', 'pending'] = ['active', 'pending'];
	showCancelInviteModal = false;
	cancelingInvite = false;
	selectedMember?: TEAM;
	isFetchingTeamMembers = false;
	isFetchingPendingInvites = false;
	deactivatingUser = false;
	searchString!: string;
	organisationId!: string;
	teams!: { pagination: PAGINATION; content: TEAM[] };
	pendingInvites!: { pagination: PAGINATION; content: TEAM[] };
	selectedFilterOption: 'active' | 'pending' = 'active';
	showOverlay = false;
	noData = false;
	noInvitesData = false;
	showFilterDropdown = false;
	updatingMember = false;
	invitingUser = false;
	showPendingInvitesDropdown = false;
	inviteUserForm: FormGroup = this.formBuilder.group({
		invitee_email: ['', Validators.compose([Validators.required, Validators.email])],
		role: this.formBuilder.group({
			type: ['super_user', Validators.required]
		})
	});
	memberForm: FormGroup = this.formBuilder.group({
		role: this.formBuilder.group({
			type: ['super_user', Validators.required]
		}),
		user_metadata: this.formBuilder.group({
			email: ['', Validators.compose([Validators.required, Validators.email])]
		})
	});
	roles = [
		{ name: 'Super User', uid: 'super_user' },
		{ name: 'Admin', uid: 'admin' },
		{ name: 'Member', uid: 'member' }
	];
	showUpdateMember = false;
	userDetails = this.privateService.getUserProfile;
	action: 'create' | 'update' = 'create';
	private rbacService = inject(RbacService);

	constructor(private generalService: GeneralService, private router: Router, private route: ActivatedRoute, private teamService: TeamsService, private formBuilder: FormBuilder, private privateService: PrivateService) {}

	async ngOnInit() {
		this.toggleFilter(this.route.snapshot.queryParams?.inviteType ?? 'active');
		if (!(await this.rbacService.userCanAccess('Team|MANAGE'))) this.inviteUserForm.disable();
	}

	async fetchTeamMembers(requestDetails?: { searchString?: string; page?: number }) {
		this.isFetchingTeamMembers = true;
		const page = requestDetails?.page || this.route.snapshot.queryParams.page || 1;
		try {
			const response = await this.privateService.getTeamMembers({ page: page, q: requestDetails?.searchString });
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
			const response = await this.teamService.getPendingTeamMembers({ page: page });
			this.pendingInvites = response.data;
			response.data.content ? (this.noInvitesData = false) : (this.noInvitesData = true);
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
		if (!this.selectedMember) return;

		this.deactivatingUser = true;
		const requestOptions = {
			memberId: this.selectedMember?.uid
		};
		try {
			const response = await this.teamService.deactivateTeamMember(requestOptions);
			this.deleteDialog.nativeElement.close();
			this.generalService.showNotification({ style: 'success', message: response.message });
			this.fetchTeamMembers();
			this.deactivatingUser = false;
		} catch (error) {
			this.deactivatingUser = false;
		}
	}

	addFilterToUrl() {
		const queryParams: any = {};
		queryParams.inviteType = this.selectedFilterOption;
		this.router.navigate([], { queryParams: Object.assign({}, queryParams) });
	}

	async inviteUser() {
		if (this.inviteUserForm.invalid) return this.inviteUserForm.markAllAsTouched();
		this.invitingUser = true;
		try {
			const response = await this.teamService.inviteUserToOrganisation(this.inviteUserForm.value);
			this.generalService.showNotification({ message: response.message, style: 'success' });
			this.inviteUserForm.reset();
			this.invitingUser = false;
			this.teamsDialog.nativeElement.close();
			this.router.navigate(['/team'], { queryParams: { inviteType: 'pending' } });
		} catch {
			this.invitingUser = false;
		}
	}

	async updateMember() {
		if (this.memberForm.invalid) return this.memberForm.markAllAsTouched();
		this.updatingMember = true;

		try {
			const response = await this.teamService.updateMember(this.memberForm.value, this.selectedMember?.uid || '');
			this.generalService.showNotification({ message: response.message, style: 'success' });
			this.memberForm.reset();
			this.updatingMember = false;
			this.action = 'create';
			this.teamsDialog.nativeElement.close();
		} catch {
			this.updatingMember = false;
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
		if (!this.selectedMember) return;

		this.cancelingInvite = true;
		try {
			const response = await this.teamService.cancelPendingInvite(this.selectedMember.uid);
			this.generalService.showNotification({ message: response.message, style: 'success' });
			this.fetchPendingTeamMembers();
			this.deleteDialog.nativeElement.close();
			this.cancelingInvite = false;
		} catch {
			this.cancelingInvite = false;
		}
	}


	showUpdateMemberModal(member: TEAM) {
		this.selectedMember = member;
		this.memberForm.patchValue(member);
		this.action = 'update';
		this.teamsDialog.nativeElement.showModal();
	}
}
