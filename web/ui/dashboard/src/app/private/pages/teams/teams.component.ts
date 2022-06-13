import { Location } from '@angular/common';
import { Component, OnInit } from '@angular/core';
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
	showInviteTeamMemberModal = this.router.url.split('/')[2]?.includes('new') && !this.router.url.split('/')[3];
	showCreateProjectModal = this.router.url.split('/')[3] === 'project';
	showTeamMemberDropdown = false;
	showTeamGroupDropdown = false;
	showSuccessModal = false;
	showDeactivateModal = false;
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

	constructor(private generalService: GeneralService, private router: Router, private route: ActivatedRoute, private teamService: TeamsService, private location: Location) {}

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
		if(!this.router.url.split('/')[2]) this.addFilterToUrl();
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

	showDropdown(id: string) {
		this.showOverlay = false;
		this.currentId == id ? (this.currentId = '') : (this.currentId = id);
	}

	goBack() {
		this.location.back();
	}
	
	addFilterToUrl() {
		const currentURLfilters = this.route.snapshot.queryParams;
		const queryParams: any = {};

		queryParams.inviteType = this.selectedFilterOption;
		this.router.navigate([], { queryParams: Object.assign({}, currentURLfilters, queryParams) });
	}
}
