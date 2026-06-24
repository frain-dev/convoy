import { Component, OnInit } from '@angular/core';
import { NgOptimizedImage } from '@angular/common';
import { ActivatedRoute, RouterModule } from '@angular/router';
import { LicensesService } from '../services/licenses/licenses.service';

@Component({
    selector: 'convoy-portal',
    imports: [RouterModule, NgOptimizedImage],
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
			name: 'Event Catalog',
			route: '/event-catalog'
		}
	];
	activeNavTab: any;
	token: string = this.route.snapshot.queryParams.token;
	ownerId: string = this.route.snapshot.queryParams.owner_id;

	constructor(private route: ActivatedRoute, private licenseService: LicensesService) {}

	ngOnInit(): void {
		this.getAuthToken();
		// Shared portal bootstrap: PortalComponent hosts every portal route, so
		// populating the license cache here (instead of only in create-endpoint)
		// means all portal pages gate on the customer's plan. A portal session
		// carries one token per page load (the sidebar links keep the same token;
		// opening a different link is a full reload), so a single fetch on init is
		// enough. setPortalLicenses() dedupes with the create-endpoint call so the
		// shared cache is loaded exactly once. By the time this runs the navigation
		// is complete and HttpService has set its token, so the fetch targets
		// /portal-api.
		void this.licenseService.setPortalLicenses();
	}

	get activeTab(): any {
		const element = document.querySelector('.nav-tab.on') as any;
		if (element) this.activeNavTab = element;
		return element || this.activeNavTab;
	}

	private getAuthToken() {
		const authToken = this.route.snapshot.queryParams.auth_token;
		if (authToken) {
			localStorage.setItem('CONVOY_PORTAL_LINK_AUTH_TOKEN', authToken);
		}
	}
}
