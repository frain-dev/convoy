import { Location } from '@angular/common';
import { Component, OnInit } from '@angular/core';
import { FormBuilder, FormControl, FormGroup, Validators } from '@angular/forms';
import { Router } from '@angular/router';
import { GROUP } from 'src/app/models/group.model';
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
	showInviteTeamMemberModal = this.router.url.split('/')[2] === 'new' && !this.router.url.split('/')[3];
	showCreateProjectModal = this.router.url.split('/')[3] === 'project';
	showTeamMemberDropdown = false;
	showTeamGroupDropdown = false;
	showSuccessModal = false;
	showDeactivateModal = false;
	selectedMember!: TEAMS;
	isFetchingTeamMembers = false;
	searchMode = false;
	deactivatingUser = false;
	searchingTeamMembers = false;
	searchString!: string;
	organisationId!: string;
	teams: TEAMS[] = [];
	currentId!: string;
	showOverlay = false;
	noData = false;

	constructor(private formBuilder: FormBuilder, private generalService: GeneralService, private router: Router, private teamService: TeamsService, private location: Location) {}

	async ngOnInit() {
		await this.fetchTeamMembers();
	}

	async fetchTeamMembers() {
		const organisation = localStorage.getItem('ORG_DETAILS');
		if (organisation) {
			const organisationDetails = JSON.parse(organisation);
			this.organisationId = organisationDetails.uid;
		}
		this.isFetchingTeamMembers = true;
		this.searchMode = false;
		try {
			const response = await this.teamService.getTeamMembers({ org_id: this.organisationId });
			if (response.data.content.length) this.teams = response.data.content;
			response.data.content.length > 0 ? (this.noData = false) : (this.noData = true);
			this.isFetchingTeamMembers = false;
		} catch {
			this.isFetchingTeamMembers = false;
		}
	}

	async searchTeam(searchInput: any) {
		this.searchMode = true;
		const searchString = searchInput;
		this.searchString = searchString;
		const requestOptions = {
			query: `?query=${searchString}`
		};
		this.searchingTeamMembers = true;
		try {
			const response = await this.teamService.searchTeamMembers(requestOptions);
			if (response.data.length) this.teams = response.data;
			this.searchingTeamMembers = false;
		} catch {
			this.searchingTeamMembers = false;
		}
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
	cancel() {
		this.router.navigate(['/team']);
	}
}
