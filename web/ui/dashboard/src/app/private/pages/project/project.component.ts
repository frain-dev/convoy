import { AfterViewInit, Component, DestroyRef, ElementRef, OnInit, QueryList, ViewChildren, inject } from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { PROJECT } from 'src/app/models/project.model';
import { PrivateService } from '../../private.service';
import { NavigationEnd, Router } from '@angular/router';
import { filter } from 'rxjs';
import { LicensesService } from 'src/app/services/licenses/licenses.service';
import { OrganisationStateService } from 'src/app/services/organisation-state/organisation-state.service';
import { BillingStrategy } from 'src/app/models/billing.model';

@Component({
    selector: 'app-project',
    templateUrl: './project.component.html',
    styleUrls: ['./project.component.scss'],
    standalone: false
})
export class ProjectComponent implements OnInit, AfterViewInit {
	sideBarItems = [
		{
			name: 'Event Deliveries',
			icon: 'events',
			route: '/events'
		},
		{
			name: 'Sources',
			icon: 'sources',
			route: '/sources'
		},
		{
			name: 'Subscriptions',
			icon: 'subscriptions',
			route: '/subscriptions'
		},
		{
			name: 'Endpoints',
			icon: 'endpoint',
			route: '/endpoints'
		}
	];
	secondarySideBarItems = [
		{
			name: 'Events Log',
			icon: 'logs',
			route: '/events-log'
		},
		{
			name: 'Meta Events',
			icon: 'meta',
			route: '/meta-events'
		}
	];
	projectDetails?: PROJECT;
	isLoadingProjectDetails: boolean = true;
	showHelpDropdown = false;
	projects: PROJECT[] = [];
	tabIndicator = { left: 0, width: 0 };
	billingStrategy: BillingStrategy = 'oss';

	@ViewChildren('navTab', { read: ElementRef }) navTabs!: QueryList<ElementRef<HTMLElement>>;
	private destroyRef = inject(DestroyRef);

	constructor(
		private privateService: PrivateService,
		private router: Router,
		public licenseService: LicensesService,
		private orgState: OrganisationStateService
	) {}

	async ngOnInit() {
		await Promise.all([this.getProjectDetails(), this.getProjects(), this.loadBillingStrategy()]);
	}

	ngAfterViewInit() {
		this.updateTabIndicator();
		this.navTabs.changes.pipe(takeUntilDestroyed(this.destroyRef)).subscribe(() => this.updateTabIndicator());
		this.router.events
			.pipe(
				filter(event => event instanceof NavigationEnd),
				takeUntilDestroyed(this.destroyRef)
			)
			.subscribe(() => this.updateTabIndicator());
	}

	private async loadBillingStrategy() {
		this.billingStrategy = await this.orgState.getBillingStrategy();
	}

	// Position the sliding tab indicator over the active nav tab. Deferred a tick
	// so routerLinkActive has applied the `on` class before we read geometry.
	private updateTabIndicator() {
		setTimeout(() => {
			const active = this.navTabs?.find(tab => tab.nativeElement.classList.contains('on'))?.nativeElement;
			if (active) this.tabIndicator = { left: active.offsetLeft, width: active.offsetWidth };
		});
	}

	async getProjectDetails() {
		this.isLoadingProjectDetails = true;

		try {
			const projectDetails = await this.privateService.getProjectDetails;
			this.projectDetails = projectDetails;
			if (this.projectDetails?.type === 'outgoing') this.sideBarItems.push({ name: 'Portal Links', icon: 'portal', route: '/portal-links' });
			this.isLoadingProjectDetails = false;
		} catch (error) {
			this.isLoadingProjectDetails = false;
		}
	}

	async getProjects() {
		try {
			const response = await this.privateService.getProjects();
			this.projects = response.data;
		} catch (error) {}
	}

	isOutgoingProject(): boolean {
		return this.projectDetails?.type === 'outgoing';
	}

	isStrokeIcon(icon: string): boolean {
		const menuIcons = ['subscriptions', 'portal', 'logs', 'meta'];
		const checkForStrokeIcon = menuIcons.some(menuIcon => icon.includes(menuIcon));

		return checkForStrokeIcon;
	}

	async getProjectCompleteDetails(project: PROJECT) {
		this.isLoadingProjectDetails = true;

		try {
			this.projectDetails = project;
			localStorage.setItem('CONVOY_PROJECT', JSON.stringify(this.projectDetails));

			if (this.projectDetails?.type === 'outgoing' && this.sideBarItems[this.sideBarItems.length - 1].icon === 'endpoint') this.sideBarItems.push({ name: 'Portal Links', icon: 'portal', route: '/portal-links' });
			if (this.projectDetails?.type === 'incoming' && this.sideBarItems[this.sideBarItems.length - 1].icon === 'portal') this.sideBarItems.pop();

			await this.privateService.getProject({ refresh: true, projectId: project.uid });
			await this.privateService.getProjectStat({ refresh: true });
			this.router.navigateByUrl('/', { skipLocationChange: true }).then(() => {
				this.router.navigate([`/projects/${project.uid}`]);
			});

			this.isLoadingProjectDetails = false;
		} catch (error) {
			this.isLoadingProjectDetails = false;
		}
	}

	get isDisabled(): boolean {
		return this.orgState.isDisabled();
	}

	get disabledOrganisationMessage(): string {
		return this.orgState.disabledOrganisationMessage(this.billingStrategy);
	}

	getProjectLimitMessage(): string {
		return this.licenseService.limitMessage('project_limit');
	}
}
