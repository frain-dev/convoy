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

	apiURL(): string {
		return `${environment.production ? location.origin : 'http://localhost:5005'}/ui`;
	}

	requestToken(): string {
		if (this.authDetails()) {
			const { username, password } = this.authDetails();
			return btoa(`${username + ':' + password}`);
		} else {
			return '';
		}
	}
}
