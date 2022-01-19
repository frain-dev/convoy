import { Component, OnInit } from '@angular/core';
import { Router } from '@angular/router';
import { environment } from 'src/environments/environment';

@Component({
	selector: 'app-dashboard',
	templateUrl: './dashboard.component.html',
	styleUrls: ['./dashboard.component.scss']
})
export class DashboardComponent implements OnInit {
	showDropdown = false;
	environment = environment;

	constructor(private router: Router) {}

	async ngOnInit() {
		await this.initDashboard();
	}

	async initDashboard() {}

	logout() {
		localStorage.removeItem('CONVOY_AUTH');
		this.router.navigateByUrl('/login');
	}

	authDetails() {
		const authDetails = localStorage.getItem('CONVOY_AUTH');
		return authDetails ? JSON.parse(authDetails) : false;
	}
}
