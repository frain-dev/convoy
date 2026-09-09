import { Component, OnInit } from '@angular/core';
import { AdminService } from '../admin.service';
import { LicensesService } from 'src/app/services/licenses/licenses.service';
import { RetentionRun } from '../runs/retention-run.model';

@Component({
	selector: 'app-retention-drops',
	templateUrl: './retention-drops.component.html',
	standalone: false
})
export class RetentionDropsComponent implements OnInit {
	runs: RetentionRun[] = [];
	isLoading = false;
	runsKnown = false;
	runsFailed = false;
	hasRetentionLicense = false;
	licenseKnown = false;

	constructor(private adminService: AdminService, private licenseService: LicensesService) {}

	async ngOnInit() {
		await this.licenseService.loadAllLicenses();
		this.hasRetentionLicense = this.licenseService.hasInstanceLicense('RetentionPolicy');
		this.licenseKnown = true;
		if (!this.hasRetentionLicense) return;

		await this.load();
	}

	async load() {
		if (this.isLoading) return;

		this.isLoading = true;
		try {
			const response = await this.adminService.listRetentionRuns();
			this.runs = response.data ?? [];
			this.runsKnown = true;
			this.runsFailed = false;
		} catch {
			this.runsKnown = false;
			this.runsFailed = true;
		} finally {
			this.isLoading = false;
		}
	}
}
