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
	appId: string = 'ed0d6b33-8201-4496-92ff-22f2993b4645';
	// appId: string = this.route.snapshot.queryParams.appId;
	// groupId: string = this.route.snapshot.queryParams.groupID;
	groupId: string = '8892c19f-733a-4959-8ded-f3c3474660c7';
	token: string = this.route.snapshot.params.token;
	apiURL = this.generalService.apiURL();

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
