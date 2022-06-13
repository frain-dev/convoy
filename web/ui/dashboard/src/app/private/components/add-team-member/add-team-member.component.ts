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
	roleTypes = [
		{ role: 'Admin', id: 'admin' },
		{ role: 'UI Admin', id: 'ui_admin' }
	];
	invitingUser = false;
	showSuccessModal = false;
	showProjectsDropdown = false;
	showAppsDropdown = false;
	organisationId!: string;
	apps: APP[] = [];
	projects: GROUP[] = [];
	filteredProjects: GROUP[] = [];
	selectedProjects: GROUP[] = [];
	noOfSelectedProjects: string = '0 Projects';
	inviteUserForm: FormGroup = this.formBuilder.group({
		invitee_email: ['', Validators.compose([Validators.required, Validators.email])],
		role: this.formBuilder.group({
			type: ['', Validators.required],
			groups: [[], Validators.required]
		})
	});

	constructor(private formBuilder: FormBuilder, private generalService: GeneralService, private addTeamService: AddTeamMemberService, private location: Location, private router:Router) {}

	async ngOnInit() {
		await Promise.all([this.getOrganisation(), this.getProjects()]);
	}

	async getProjects() {
		try {
			const response = await this.addTeamService.getProjects({ pageNo: 1 });
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

	searchProjects(searchInput: any) {
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

	async inviteUser() {
		const groupIds = this.selectedProjects.map(item => item.uid);
		this.inviteUserForm.patchValue({
			role: { groups: groupIds }
		});
		if (this.inviteUserForm.invalid) {
			(<any>this.inviteUserForm).values(this.inviteUserForm.controls).forEach((control: FormControl) => {
				control?.markAsTouched();
			});
			return;
		}
		this.invitingUser = true;
		try {
			const response = await this.addTeamService.inviteUserToOrganisation({ org_id: this.organisationId, body: this.inviteUserForm.value });
			if (response.data) this.showSuccessModal = true;
			this.inviteUserForm.reset();
			this.invitingUser = false;
			this.goBack();
		} catch {
			this.invitingUser = false;
		}
	}

	selectProject(project: GROUP) {
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

	getOrganisation() {
		const organisation = localStorage.getItem('ORG_DETAILS');
		if (organisation) {
			const organisationDetails = JSON.parse(organisation);
			this.organisationId = organisationDetails.uid;
		}
	}

	createNewProject(){
		this.router.navigateByUrl('/team/new/project');
	}

	goBack() {
		this.location.back();
	}
}
