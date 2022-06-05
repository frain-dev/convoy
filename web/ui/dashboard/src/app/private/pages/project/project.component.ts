import { Component, HostListener, OnInit } from '@angular/core';
import { ActivatedRoute } from '@angular/router';
import { PrivateService } from '../../private.service';
import { ProjectService } from './project.service';

@Component({
	selector: 'app-project',
	templateUrl: './project.component.html',
	styleUrls: ['./project.component.scss']
})
export class ProjectComponent implements OnInit {
	screenWidth = window.innerWidth;
	sideBarItems = [
		{
			name: 'Events',
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
			name: 'Apps',
			icon: 'apps',
			route: '/apps'
		}
	];
	shouldShowFullSideBar = true;

	constructor(private route: ActivatedRoute, private privateService: PrivateService) {
		this.privateService.activeProjectId = this.route.snapshot.params.id;
	}

	ngOnInit() {
		this.checkScreenSize();
	}

	checkScreenSize() {
		this.screenWidth > 1150 ? (this.shouldShowFullSideBar = true) : (this.shouldShowFullSideBar = false);
	}

	@HostListener('window:resize', ['$event'])
	onWindowResize() {
		this.screenWidth = window.innerWidth;
		this.checkScreenSize();
	}
}
