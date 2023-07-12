import { Component, OnInit } from '@angular/core';
import { Router } from '@angular/router';
import { PrivateService } from 'src/app/private/private.service';
import { GeneralService } from 'src/app/services/general/general.service';

@Component({
	selector: 'app-settings',
	templateUrl: './settings.component.html',
	styleUrls: ['./settings.component.scss']
})
export class SettingsComponent implements OnInit {
	isloading = false;
	showDeleteProjectModal = false;

	constructor(public privateService: PrivateService, private router: Router, private generalService: GeneralService) {}

	ngOnInit() {}

	async deleteProject() {
		this.isloading = true;

		try {
			await this.privateService.deleteProject();
			this.generalService.showNotification({ message: 'Project deleted successfully', style: 'success' });
			this.router.navigateByUrl('/').then(() => {
				window.location.reload();
			});
			this.isloading = false;
		} catch (error) {
			this.isloading = false;
		}
	}
}
