import { Component, OnInit } from '@angular/core';
import { Router } from '@angular/router';
import { PrivateService } from 'src/app/private/private.service';
import { GeneralService } from 'src/app/services/general/general.service';
import { SettingsService } from './settings.service';

@Component({
	selector: 'app-settings',
	templateUrl: './settings.component.html',
	styleUrls: ['./settings.component.scss']
})
export class SettingsComponent implements OnInit {
	isloading = false;
	showDeleteProjectModal = false;

	constructor(public privateService: PrivateService, private settingsService: SettingsService, private router: Router, private generalService: GeneralService) {}

	ngOnInit(): void {}

	async deleteProject() {
		this.isloading = true;

		try {
			await this.settingsService.deleteProject();
			this.generalService.showNotification({ message: 'Project deleted successfully', style: 'success' });
			this.router.navigateByUrl('/projects');
			this.isloading = false;
		} catch (error) {
			this.isloading = false;
		}
	}
}
