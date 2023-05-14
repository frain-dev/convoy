import { Component, HostListener, OnInit } from '@angular/core';
import { PROJECT } from 'src/app/models/project.model';
import { PrivateService } from '../../private.service';

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
		},
		{
			name: 'Portal Links',
			icon: 'portal',
			route: '/portal-links'
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
	projectDetails!: PROJECT;
	isLoadingProjectDetails: boolean = true;
	showHelpDropdown = false;

	constructor(private privateService: PrivateService) {}

	ngOnInit() {
		Promise.all([this.checkScreenSize(), this.getProjectDetails()]);
	}

	async getProjectDetails() {
		this.isLoadingProjectDetails = true;

		try {
			const projectDetails = await this.privateService.getProjectDetails();
			this.projectDetails = projectDetails.data;
			if (this.projectDetails.type === 'incoming') this.sideBarItems.splice(4, 1);
			this.isLoadingProjectDetails = false;
		} catch (error) {
			this.isLoadingProjectDetails = false;
		}
	}

	isOutgoingProject(): boolean {
		return this.projectDetails.type === 'outgoing';
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
}
