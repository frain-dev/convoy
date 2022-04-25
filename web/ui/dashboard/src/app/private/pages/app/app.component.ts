import { Component, OnInit } from '@angular/core';
import { ActivatedRoute, Router } from '@angular/router';
import { GeneralService } from 'src/app/services/general/general.service';

@Component({
	selector: 'app-app',
	templateUrl: './app.component.html',
	styleUrls: ['./app.component.scss']
})
export class AppComponent implements OnInit {
	showDropdown = false;
	token: string = this.route.snapshot.params.token;
	apiURL = this.generalService.apiURL();
	appId: string = this.route.snapshot.queryParams.appId
	constructor(private router: Router, private route: ActivatedRoute, private generalService: GeneralService) {}

	ngOnInit() {}

	logout() {
		localStorage.removeItem('CONVOY_AUTH');
		this.router.navigateByUrl('/login');
	}

	authDetails() {
		const authDetails = localStorage.getItem('CONVOY_AUTH');
		return authDetails ? JSON.parse(authDetails) : false;
	}
}
