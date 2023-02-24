import { Component, HostListener, OnInit } from '@angular/core';
import { ActivatedRoute } from '@angular/router';
import { GROUP } from 'src/app/models/group.model';
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
		},
		{
			name: 'Events Log',
			icon: 'logs',
			route: '/events-log'
		}
	];
	shouldShowFullSideBar = true;
	projectDetails!: GROUP;
	isLoadingProjectDetails: boolean = true;
	showHelpDropdown = false;

	constructor(private route: ActivatedRoute, private privateService: PrivateService) {
		const data: any = { uid: this.route.snapshot.params.id, ...this.privateService.activeProjectDetails };
		this.privateService.activeProjectDetails = data;
		this.getSubscriptions();
	}

	ngOnInit() {
		this.checkScreenSize();
		this.getProjectDetails();
	}

	async getProjectDetails() {
		this.isLoadingProjectDetails = true;

		try {
			const projectDetails = await this.privateService.getProjectDetails();
			this.projectDetails = projectDetails.data;
			localStorage.setItem('PROJECT_CONFIG', JSON.stringify(projectDetails.data?.config));
			if (this.projectDetails.type === 'incoming') this.sideBarItems.splice(4, 1);
			this.isLoadingProjectDetails = false;
		} catch (error) {
			this.isLoadingProjectDetails = false;
		}
	}

	async getSubscriptions() {
		try {
			const subscriptionsResponse = await this.privateService.getSubscriptions({ page: 1 });
			subscriptionsResponse.data?.content?.length === 0 ? localStorage.setItem('isActiveProjectConfigurationComplete', 'false') : localStorage.setItem('isActiveProjectConfigurationComplete', 'true');
		} catch (error) {
			return error;
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
		const menuIcons = ['subscriptions', 'portal', 'logs'];
		const checkForStrokeIcon = menuIcons.some(menuIcon => icon.includes(menuIcon));

		return checkForStrokeIcon;
	}
}
