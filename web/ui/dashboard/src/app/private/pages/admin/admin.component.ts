import { Component, OnInit, ViewChild } from '@angular/core';
import { ActivatedRoute, Router } from '@angular/router';
import { ConfigurationsComponent } from '../settings/configurations/configurations.component';

export type ADMIN_PAGE = 'configurations' | 'feature flags' | 'circuit breaker config' | 'resend events' | 'queue monitoring' | 'table partitions' | 'table indexes';

@Component({
    selector: 'app-admin',
    templateUrl: './admin.component.html',
    styleUrls: ['./admin.component.scss'],
    standalone: false
})
export class AdminComponent implements OnInit {
	activePage: ADMIN_PAGE = 'configurations';
	adminMenu: { name: ADMIN_PAGE; icon: string; svg: 'stroke' | 'fill' }[] = [
		{ name: 'configurations', icon: 'settings', svg: 'fill' },
		{ name: 'feature flags', icon: 'settings', svg: 'fill' },
		{ name: 'circuit breaker config', icon: 'shield', svg: 'fill' },
		{ name: 'resend events', icon: 'retry', svg: 'fill' },
		{ name: 'queue monitoring', icon: 'logs', svg: 'stroke' },
		{ name: 'table partitions', icon: 'table-grid', svg: 'fill' },
		{ name: 'table indexes', icon: 'key', svg: 'fill' }
	];

	@ViewChild(ConfigurationsComponent) configurations?: ConfigurationsComponent;

	constructor(private route: ActivatedRoute, private router: Router) {}

	// Every class is written out in full because Tailwind reads these files as text:
	// a name assembled from parts at runtime is never generated, and the missing
	// utility is silent. stroke-new.text-primary appears nowhere else in the app, so
	// a stroke icon asking for it would render with no stroke at all. Colours match
	// the label beside them, leaving the selected row's background to mark it.
	iconClass(menu: { svg: 'stroke' | 'fill' }, active: boolean): string {
		if (menu.svg === 'fill') {
			return active ? 'fill-new.text-primary' : 'fill-new.text-secondary';
		}
		// A symbol drawn as outlines still fills black by default, so clear it.
		return active ? 'fill-none stroke-new.text-primary' : 'fill-none stroke-new.text-secondary';
	}

	ngOnInit() {
		// Set active page from URL query parameter
		const requestedPage = this.route.snapshot.queryParams?.activePage ?? 'configurations';
		this.toggleActivePage(requestedPage);
	}

	// Checked against the menu rather than a second list of the same names, so a
	// page added above cannot be one an ?activePage link silently falls back from.
	toggleActivePage(page: string) {
		const known = this.adminMenu.some(menu => menu.name === page);
		const next = known ? (page as ADMIN_PAGE) : 'configurations';
		if (next === this.activePage) {
			this.addPageToUrl();
			return;
		}
		if (!this.confirmLeaveConfigurations(next)) {
			return;
		}
		this.activePage = next;
		this.addPageToUrl();
	}

	leaveAdmin(event: Event) {
		// Always take navigation: routerLink on the same anchor still fires after
		// preventDefault alone, so Cancel would discard unsaved Configurations.
		event.preventDefault();
		event.stopPropagation();
		if (!this.confirmLeaveConfigurations(null)) {
			return;
		}
		void this.router.navigate(['/projects']);
	}

	// next null means leaving Admin entirely (back to projects).
	private confirmLeaveConfigurations(next: ADMIN_PAGE | null): boolean {
		if (this.activePage !== 'configurations') {
			return true;
		}
		if (next === 'configurations') {
			return true;
		}
		if (!this.configurations?.hasUnsavedChanges) {
			return true;
		}
		return window.confirm('You have unsaved configuration changes. Leave without saving?');
	}

	// The tab is written back to the URL so a reload lands where the operator
	// was. Reading the param without writing it is what sent every refresh to
	// whichever tab the URL happened to be carrying. replaceUrl keeps the back
	// button pointing outside admin rather than at each tab in turn, and
	// merging leaves params the tabs themselves own, like the queue page's
	// full-screen flag.
	private addPageToUrl() {
		this.router.navigate([], {
			relativeTo: this.route,
			queryParams: { activePage: this.activePage },
			queryParamsHandling: 'merge',
			replaceUrl: true
		});
	}
}
