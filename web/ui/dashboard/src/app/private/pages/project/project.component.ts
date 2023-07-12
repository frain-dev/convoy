import { Component, HostListener, OnInit } from '@angular/core';
import { PROJECT } from 'src/app/models/project.model';
import { PrivateService } from '../../private.service';
import { Router } from '@angular/router';

@Component({
	selector: 'app-project',
	templateUrl: './project.component.html',
	styleUrls: ['./project.component.scss']
})
export class ProjectComponent implements OnInit {
	screenWidth = window.innerWidth;
	sideBarItems = [
		{
			name: 'Event Deliveries',
			icon: 'events',
			route: '/events'
		},
		{
			name: 'Sources',
			icon: 'sources',
			route: '/sources'
		},
		{
			name: 'Subscriptions',
			icon: 'subscriptions',
			route: '/subscriptions'
		},
		{
			name: 'Endpoints',
			icon: 'endpoint',
			route: '/endpoints'
		}
	];
	secondarySideBarItems = [
		{
			name: 'Events Log',
			icon: 'logs',
			route: '/events-log'
		},
		{
			name: 'Meta Events',
			icon: 'meta',
			route: '/meta-events'
		}
	];
	shouldShowFullSideBar = true;
	projectDetails?: PROJECT;
	isLoadingProjectDetails: boolean = true;
	showHelpDropdown = false;
	projects: PROJECT[] = [];

	constructor(private privateService: PrivateService, private router: Router) {}

	ngOnInit() {
		Promise.all([this.checkScreenSize(), this.getProjectDetails(), this.getProjects()]);
	}

	async getProjectDetails() {
		this.isLoadingProjectDetails = true;

		try {
			const projectDetails = await this.privateService.getProjectDetails;
			this.projectDetails = projectDetails;
			if (this.projectDetails?.type === 'outgoing') this.sideBarItems.push({ name: 'Portal Links', icon: 'portal', route: '/portal-links' });
			this.isLoadingProjectDetails = false;
		} catch (error) {
			this.isLoadingProjectDetails = false;
		}
	}

	async getProjects() {
		try {
			const response = await this.privateService.getProjects();
			this.projects = response.data;
		} catch (error) {}
	}

	isOutgoingProject(): boolean {
		return this.projectDetails?.type === 'outgoing';
	}

	checkScreenSize() {
		this.screenWidth > 1150 ? (this.shouldShowFullSideBar = true) : (this.shouldShowFullSideBar = false);
	}

	@HostListener('window:resize', ['$event'])
	onWindowResize() {
		this.screenWidth = window.innerWidth;
		this.checkScreenSize();
	}

	isStrokeIcon(icon: string): boolean {
		const menuIcons = ['subscriptions', 'portal', 'logs', 'meta'];
		const checkForStrokeIcon = menuIcons.some(menuIcon => icon.includes(menuIcon));

		return checkForStrokeIcon;
	}

	async getProjectCompleteDetails(project: PROJECT) {
		this.isLoadingProjectDetails = true;

		try {
			this.projectDetails = project;
			localStorage.setItem('CONVOY_PROJECT', JSON.stringify(this.projectDetails));

			if (this.projectDetails?.type === 'outgoing' && this.sideBarItems[this.sideBarItems.length - 1].icon === 'endpoint') this.sideBarItems.push({ name: 'Portal Links', icon: 'portal', route: '/portal-links' });
			if (this.projectDetails?.type === 'incoming' && this.sideBarItems[this.sideBarItems.length - 1].icon === 'portal') this.sideBarItems.pop();
			await this.privateService.getProjectStat({ refresh: true });
			this.isLoadingProjectDetails = false;
			this.router.navigate([`/projects/${project.uid}`]);
		} catch (error) {
			this.isLoadingProjectDetails = false;
		}
	}
}
