import { Component, OnInit } from '@angular/core';
import { ActivatedRoute, Router } from '@angular/router';
import { HttpService } from 'src/app/services/http/http.service';

@Component({
	selector: 'app-app',
	templateUrl: './app.component.html',
	styleUrls: ['./app.component.scss']
})
export class AppComponent implements OnInit {
	showDropdown = false;
	appId: string = this.route.snapshot.queryParams.appId;
	groupId: string = this.route.snapshot.queryParams.groupId;
	token: string = this.route.snapshot.params.token;

	constructor(private router: Router, private route: ActivatedRoute) {}

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
