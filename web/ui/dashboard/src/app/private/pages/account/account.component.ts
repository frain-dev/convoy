import { Component, OnInit } from '@angular/core';
import { ActivatedRoute, Router } from '@angular/router';

export type ACCOUNT_SETTINGS = 'profile' | 'security' | 'personal access tokens';

@Component({
	selector: 'app-account',
	templateUrl: './account.component.html',
	styleUrls: ['./account.component.scss']
})
export class AccountComponent implements OnInit {
	activePage: ACCOUNT_SETTINGS = 'profile';
	settingsMenu: { name: ACCOUNT_SETTINGS; icon: string; svg: 'fill' | 'stroke' }[] = [
		{ name: 'profile', icon: 'profile', svg: 'stroke' },
		{ name: 'security', icon: 'security', svg: 'fill' },
		{ name: 'personal access tokens', icon: 'key', svg: 'fill' }
	];

	constructor(private router: Router, private route: ActivatedRoute) {}

	ngOnInit() {
		this.toggleActivePage(this.route.snapshot.queryParams?.activePage ?? 'profile');
	}

	toggleActivePage(activePage: ACCOUNT_SETTINGS) {
		this.activePage = activePage;
		if (!this.router.url.split('/')[2]) this.addPageToUrl();
	}

	addPageToUrl() {
		const queryParams: any = {};
		queryParams.activePage = this.activePage;
		this.router.navigate([], { queryParams: Object.assign({}, queryParams) });
	}
}
