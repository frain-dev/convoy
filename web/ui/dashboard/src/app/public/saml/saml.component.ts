import { Component, OnInit } from '@angular/core';
import { CommonModule } from '@angular/common';
import { LoaderModule } from 'src/app/private/components/loader/loader.module';
import { SamlService } from './saml.service';
import { ActivatedRoute, Router } from '@angular/router';
import { PrivateService } from 'src/app/private/private.service';

@Component({
	selector: 'convoy-saml',
	standalone: true,
	imports: [CommonModule, LoaderModule],
	templateUrl: './saml.component.html',
	styleUrls: ['./saml.component.scss']
})
export class SamlComponent implements OnInit {
	token: string = this.route.snapshot.queryParams['token'] ?? this.route.snapshot.queryParams['sso_token'];

	constructor(private samlService: SamlService, private route: ActivatedRoute, private router: Router, private privateService: PrivateService) {}

	ngOnInit() {
		this.redeemToken();
	}

	async redeemToken() {
		if (!this.token) {
			const authType = localStorage.getItem('AUTH_TYPE');
			this.router.navigateByUrl(authType === 'login' ? '/login' : '/signup');
			return;
		}
		try {
			const response = await this.samlService.redeemSSOToken(this.token);
			localStorage.setItem('CONVOY_AUTH', JSON.stringify(response.data));
			localStorage.setItem('CONVOY_AUTH_TOKENS', JSON.stringify(response.data.token));
			await this.getOrganisations();

			if (typeof window !== 'undefined' && window.opener) {
				const pathname = window.location.pathname;
				const appRoot = pathname.replace(/\/sso\/callback(\?.*)?$/i, '').replace(/\/$/, '') || '';
				const projectsUrl = window.location.origin + (appRoot ? appRoot + '/projects' : '/projects');
				window.opener.location.href = projectsUrl;
				window.close();
				return;
			}
			this.router.navigateByUrl('/');
		} catch (err) {
			const authType = localStorage.getItem('AUTH_TYPE');
			this.router.navigateByUrl(authType === 'login' ? '/login' : '/signup');
		}
	}

	async getOrganisations() {
		try {
			await this.privateService.getOrganizations({ refresh: true });
			return;
		} catch (error) {
			return error;
		}
	}
}
