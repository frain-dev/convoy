import { Component, OnInit } from '@angular/core';
import { Router } from '@angular/router';
import { GeneralService } from 'src/app/services/general/general.service';

@Component({
	selector: 'app-dashboard',
	templateUrl: './dashboard.component.html',
	styleUrls: ['./dashboard.component.scss']
})
export class DashboardComponent implements OnInit {
	showDropdown = false;
	apiURL = this.generalService.apiURL();

	constructor(private generalService: GeneralService, private router: Router) {}

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

	requestToken(): string {
		if (this.authDetails()) {
			const { username, password } = this.authDetails();
			return btoa(`${username + ':' + password}`);
		} else {
			return '';
		}
	}
}
