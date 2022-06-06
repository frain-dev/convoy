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
	tableHead: string[] = ['Name', 'Role', 'Projects', 'Status', ''];
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
	teams: TEAMS[] = [];
	projects: GROUP[] = [];
	filteredProjects: GROUP[] = [];
	selectedProjects: GROUP[] = [];
	noOfSelectedProjects: string = '0 Projects';
	invitingUser = false;
	currentId!: string;
	showOverlay = false;
	noData = false;
	inviteUserForm: FormGroup = this.formBuilder.group({
		firstname: ['', Validators.required],
		lastname: ['', Validators.required],
		email: ['', Validators.compose([Validators.required, Validators.email])],
		role: ['', Validators.required],
		groups: [[], Validators.required]
	});

	constructor(private formBuilder: FormBuilder, private generalService: GeneralService, private router: Router, private teamService: TeamsService, private location: Location) {}

	async ngOnInit() {
		await Promise.all([this.fetchTeamMembers(), this.getProjects()]);
    console.log(this.router.url.split('/')[3])
    console.log(this.showCreateProjectModal)
    console.log(this.showInviteTeamMemberModal)
	}

	async fetchTeamMembers() {
		this.isFetchingTeamMembers = true;
		this.searchMode = false;
		try {
			const response = await this.teamService.getTeamMembers();
			if (response.data.length) this.teams = response.data;
			// response.data.length > 0 ? (this.noData = false) : (this.noData = true);
			this.isFetchingTeamMembers = false;
		} catch {
			this.isFetchingTeamMembers = false;
		}
	}

	async getProjects() {
		try {
			const response = await this.teamService.getProjects();
			const projectsAvailable = response.data;
			projectsAvailable.forEach((element: GROUP) => {
				this.projects.push({
					...element,
					selected: false
				});
			});
			this.filteredProjects = this.projects;
		} catch {}
	}

	searchGroup(searchInput: any) {
		const searchString = searchInput.target.value;
		if (searchString) {
			this.filteredProjects = this.projects.filter(element => {
				let filteredProjects = element.name.toLowerCase();
				return filteredProjects.includes(searchString);
			});
		} else {
			this.filteredProjects = this.projects;
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

	selectGroup(project: GROUP) {
		const id = project.uid;
		if (this.selectedProjects?.length) {
			const projectExists = this.selectedProjects.find(item => item.uid == id);
			if (projectExists) {
				this.selectedProjects = this.selectedProjects.filter(project => project.uid != id);
				this.filteredProjects.forEach((item: GROUP) => {
					if (item.uid == id) item.selected = false;
				});
			} else {
				this.selectedProjects.push(project);
				this.filteredProjects.forEach((item: GROUP) => {
					if (item.uid == id) item.selected = true;
				});
			}
		} else {
			this.selectedProjects.push(project);
			this.filteredProjects.forEach((item: GROUP) => {
				if (item.uid == id) item.selected = true;
			});
		}

		this.noOfSelectedProjects = `${this.selectedProjects?.length} project${this.selectedProjects?.length == 1 ? '' : 's'}`;
	}
	async inviteUser() {
		const groupIds = this.selectedProjects.map(item => item.uid);
		this.inviteUserForm.patchValue({
			groups: groupIds
		});
		if (this.inviteUserForm.invalid) {
			(<any>this.inviteUserForm).values(this.inviteUserForm.controls).forEach((control: FormControl) => {
				control?.markAsTouched();
			});
			return;
		}
		this.invitingUser = true;
		try {
			const response = await this.teamService.inviteUserToOrganisation(this.inviteUserForm.value);
			if (response.data) this.showSuccessModal = true;
			this.showInviteTeamMemberModal = false;
			this.inviteUserForm.reset();
			this.fetchTeamMembers();
			this.invitingUser = false;
		} catch {
			this.invitingUser = false;
		}
	}

	async deactivateMember() {
		this.deactivatingUser = true;
		const requestOptions = {
			memberId: this.selectedMember?.id
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
		this.currentId == id ? (this.currentId = '') : (this.currentId = id);
	}

	closeCreateGroupModal(fetchProjects: boolean) {
		this.showCreateProjectModal = false;
		if (fetchProjects) this.getProjects();
	}

	goBack() {
		this.location.back();
	}
	cancel() {
		this.router.navigate(['/team']);
	}
}
