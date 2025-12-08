import { Component, OnInit } from '@angular/core';
import { ActivatedRoute } from '@angular/router';

export type ADMIN_PAGE = 'feature flags' | 'circuit breaker config' | 'resend events';

@Component({
	selector: 'app-admin',
	templateUrl: './admin.component.html',
	styleUrls: ['./admin.component.scss']
})
export class AdminComponent implements OnInit {
	activePage: ADMIN_PAGE = 'feature flags';
	adminMenu: { name: ADMIN_PAGE; icon: string; svg: 'stroke' | 'fill' }[] = [
		{ name: 'feature flags', icon: 'settings', svg: 'fill' },
		{ name: 'circuit breaker config', icon: 'shield', svg: 'fill' },
		{ name: 'resend events', icon: 'retry', svg: 'fill' }
	];

	constructor(private route: ActivatedRoute) {}

	ngOnInit() {
		// Set active page from URL query parameter
		const requestedPage = this.route.snapshot.queryParams?.activePage ?? 'feature flags';
		this.toggleActivePage(requestedPage);
	}

	toggleActivePage(page: string) {
		if (page === 'feature flags' || page === 'circuit breaker config' || page === 'resend events') {
			this.activePage = page as ADMIN_PAGE;
		} else {
			this.activePage = 'feature flags';
		}
	}
}
