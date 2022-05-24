import { Component, OnInit } from '@angular/core';
import { ActivatedRoute } from '@angular/router';
import { ProjectService } from './project.service';

@Component({
	selector: 'app-project',
	templateUrl: './project.component.html',
	styleUrls: ['./project.component.scss']
})
export class ProjectComponent implements OnInit {
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

	constructor(private route: ActivatedRoute, private projectService: ProjectService) {
		this.projectService.activeProject = this.route.snapshot.params.id;
	}

	ngOnInit(): void {}
}
