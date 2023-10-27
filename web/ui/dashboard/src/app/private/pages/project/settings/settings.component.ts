import { Component, ElementRef, OnInit, ViewChild } from '@angular/core';
import { Router } from '@angular/router';
import { PrivateService } from 'src/app/private/private.service';
import { GeneralService } from 'src/app/services/general/general.service';

@Component({
	selector: 'app-settings',
	templateUrl: './settings.component.html',
	styleUrls: ['./settings.component.scss']
})
export class SettingsComponent implements OnInit {
	@ViewChild('deleteDialog', { static: true }) deleteDialog!: ElementRef<HTMLDialogElement>;

	isloading = false;

	constructor(public privateService: PrivateService, private generalService: GeneralService, public router: Router) {}

	ngOnInit() {}

	async deleteProject() {
		this.deleteDialog.nativeElement.close();
		this.isloading = true;
		document.body.scrollTop = document.documentElement.scrollTop = 0;

		try {
			await this.privateService.deleteProject();
			this.generalService.showNotification({ message: 'Project deleted successfully', style: 'success' });
			this.router.navigateByUrl('/projects');

			this.isloading = false;
		} catch (error) {
			this.isloading = false;
		}
	}
}
