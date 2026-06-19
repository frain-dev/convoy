import { Component, OnInit } from '@angular/core';
import { PROJECT } from 'src/app/models/project.model';
import { PrivateService } from '../../private.service';
import { Router } from '@angular/router';
import { LicensesService } from 'src/app/services/licenses/licenses.service';
import { OrganisationStateService } from 'src/app/services/organisation-state/organisation-state.service';
import { BillingStrategy } from 'src/app/models/billing.model';

@Component({
    selector: 'app-projects',
    templateUrl: './projects.component.html',
    styleUrls: ['./projects.component.scss'],
    standalone: false
})
export class ProjectsComponent implements OnInit {
	projects: PROJECT[] = [];
	isLoadingProjects = false;
	projectsLoaderIndex: number[] = [0, 1, 2, 3, 4];
	showOrganisationModal = false;
	isLoadingProject: boolean = false;
	billingStrategy: BillingStrategy = 'oss';

	constructor(
		private privateService: PrivateService,
		private router: Router,
		public licenseService: LicensesService,
		private orgState: OrganisationStateService
	) {
		this.privateService.projects$.subscribe(projects => (this.projects = projects.data));
	}

	async ngOnInit() {
		await Promise.all([this.getProjects(), this.loadBillingStrategy()]);
	}

	private async loadBillingStrategy() {
		this.billingStrategy = await this.orgState.getBillingStrategy();
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
		return this.orgState.isDisabled();
	}

	/** Message for the card layout (grid) overlay. */
	getProjectLimitMessage(): string {
		if (this.isProjectLimitReached) {
			return this.projectLimitReachedMessage;
		}

		if (this.canShowProjectLimitUpgrade) {
			return 'Available on Business';
		}

		return '';
	}

	/** Message for the empty state only (improved, billing-aware copy). */
	getProjectLimitMessageForEmptyState(): string {
		if (this.isProjectLimitReached) {
			return this.projectLimitReachedMessage;
		}

		if (this.canShowProjectLimitUpgrade) {
			return 'Upgrade your plan to create more projects';
		}

		return '';
	}

	get shouldBlockProjectCreation(): boolean {
		return this.isDisabled || this.isProjectLimitReached || this.canShowProjectLimitUpgrade;
	}

	private get isProjectLimitReached(): boolean {
		return this.licenseService.isLimitAvailable('project_limit') && this.licenseService.isLimitReached('project_limit');
	}

	private get canShowProjectLimitUpgrade(): boolean {
		return this.canOpenBillingForProjectLimit &&
			!this.licenseService.hasLicense('project_limit') &&
			!this.licenseService.isLimitAvailable('project_limit');
	}

	private get projectLimitReachedMessage(): string {
		return this.licenseService.limitReachedMessage('project_limit');
	}

	get canOpenBillingForProjectLimit(): boolean {
		return this.billingStrategy === 'cloud' || this.billingStrategy === 'licensed_self_hosted';
	}

	get disabledOrganisationMessage(): string {
		return this.orgState.disabledOrganisationMessage(this.billingStrategy);
	}
}
