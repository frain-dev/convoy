import { Component, OnInit } from '@angular/core';
import { CommonModule } from '@angular/common';
import { ActivatedRoute, RouterModule } from '@angular/router';

@Component({
	selector: 'convoy-portal',
	standalone: true,
	imports: [CommonModule, RouterModule],
	templateUrl: './portal.component.html',
	styleUrls: ['./portal.component.scss']
})
export class PortalComponent implements OnInit {
	sideBarItems = [
		{
			name: 'Event Deliveries',
			route: '/'
		},
		{
			name: 'Endpoints',
			route: '/endpoints'
		},
		{
			name: 'Subscriptions',
			route: '/subscriptions'
		}
	];
	activeNavTab: any;
	token: string = this.route.snapshot.queryParams.token;

	constructor(private route: ActivatedRoute) {}

	ngOnInit(): void {}

	get activeTab(): any {
		const element = document.querySelector('.nav-tab.on') as any;
		if (element) this.activeNavTab = element;
		return element || this.activeNavTab;
	}
}
