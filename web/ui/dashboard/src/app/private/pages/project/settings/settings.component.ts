import { Component, OnInit } from '@angular/core';
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

	constructor(public privateService: PrivateService, private generalService: GeneralService) {}

	ngOnInit() {}

	async deleteProject() {
		this.showDeleteProjectModal = false;
		this.isloading = true;
		document.body.scrollTop = document.documentElement.scrollTop = 0;

		try {
			await this.privateService.deleteProject();
			await this.privateService.getProjectsHelper({ refresh: true });
			this.generalService.showNotification({ message: 'Project deleted successfully', style: 'success' });
			this.isloading = false;
		} catch (error) {
			this.isloading = false;
		}
	}
}
