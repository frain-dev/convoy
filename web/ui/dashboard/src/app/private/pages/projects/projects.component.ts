import { Component, OnInit } from '@angular/core';
import { PROJECT } from 'src/app/models/project.model';
import { PrivateService } from '../../private.service';
import { Router } from '@angular/router';
import { LicensesService } from 'src/app/services/licenses/licenses.service';
import { HttpService } from 'src/app/services/http/http.service';

@Component({
	selector: 'app-projects',
	templateUrl: './projects.component.html',
	styleUrls: ['./projects.component.scss']
})
export class ProjectsComponent implements OnInit {
	projects: PROJECT[] = [];
	isLoadingProjects = false;
	projectsLoaderIndex: number[] = [0, 1, 2, 3, 4];
	showOrganisationModal = false;
	isLoadingProject: boolean = false;
	/** True when instance uses billing (instance license path); false for community/OSS. Matches backend Billing.Enabled. */
	billingEnabled = false;

	constructor(
		private privateService: PrivateService,
		private router: Router,
		public licenseService: LicensesService,
		private httpService: HttpService
	) {
		this.privateService.projects$.subscribe(projects => (this.projects = projects.data));
	}

	async ngOnInit() {
		await Promise.all([this.getProjects(), this.checkBillingEnabled()]);
	}

	private async checkBillingEnabled() {
		try {
			const response = await this.httpService.request({
				url: '/billing/enabled',
				method: 'get',
				hideNotification: true
			});
			this.billingEnabled = response.data?.enabled === true;
		} catch {
			this.billingEnabled = false;
		}
	}

	async getProject(projectId: string) {
		this.isLoadingProjects = true;

		try {
			await this.privateService.getProject({ refresh: true, projectId });
			await this.privateService.getProjectStat({ refresh: true });

			this.router.navigate([`/projects/${projectId}`]);
			this.isLoadingProjects = false;
		} catch (error) {
			this.isLoadingProjects = false;
		}
	}

	async getProjects(): Promise<any> {
		this.isLoadingProjects = true;

		try {
			const response = await this.privateService.getProjects();
			this.projects = response.data;
			this.isLoadingProjects = false;
		} catch (error) {
			this.isLoadingProjects = false;
			return error;
		}
	}

	get isDisabled(): boolean {
		const org = localStorage.getItem('CONVOY_ORG');
		if (!org) return false;
		try {
			const organisationDetails = JSON.parse(org);
			return organisationDetails.disabled_at != null && organisationDetails.disabled_at !== undefined;
		} catch {
			return false;
		}
	}

	/** Message for the card layout (grid) overlay. */
	getProjectLimitMessage(): string {
		if (!this.licenseService.hasLicense('project_limit')) {
			if (!this.licenseService.isLimitAvailable('project_limit')) {
				return 'Available on Business';
			}
			if (this.licenseService.isLimitAvailable('project_limit') && this.licenseService.isLimitReached('project_limit')) {
				const limitInfo = this.licenseService.getLimitInfo('project_limit');
				const current = limitInfo?.current ?? 0;
				const limit = limitInfo?.limit === -1 ? '∞' : (limitInfo?.limit ?? 0);
				return `Limit reached (${current}/${limit})`;
			}
		}
		return '';
	}

	/** Message for the empty state only (improved, billing-aware copy). */
	getProjectLimitMessageForEmptyState(): string {
		if (!this.licenseService.hasLicense('project_limit')) {
			if (!this.licenseService.isLimitAvailable('project_limit')) {
				return this.billingEnabled
					? 'Upgrade to Business plan to create more projects'
					: 'Project limit reached';
			}
			if (this.licenseService.isLimitAvailable('project_limit') && this.licenseService.isLimitReached('project_limit')) {
				const limitInfo = this.licenseService.getLimitInfo('project_limit');
				const current = limitInfo?.current ?? 0;
				const limit = limitInfo?.limit === -1 ? '∞' : (limitInfo?.limit ?? 0);
				return `Limit reached (${current}/${limit})`;
			}
		}
		return '';
	}
}
