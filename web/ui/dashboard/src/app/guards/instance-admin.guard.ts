import { Injectable } from '@angular/core';
import { Router } from '@angular/router';
import { RbacService } from '../services/rbac/rbac.service';
import { PrivateService } from '../private/private.service';

@Injectable({
	providedIn: 'root'
})
export class InstanceAdminGuard  {
	constructor(
		private rbacService: RbacService,
		private privateService: PrivateService,
		private router: Router
	) {}

	async canActivate(): Promise<boolean> {
		try {
			// Membership is org-scoped. A full reload of /admin runs this guard
			// before the shell has fetched organisations, and getUserRole then
			// reads an empty cache as PROJECT_VIEWER.
			await this.privateService.getOrganizations();
			const userRole = await this.rbacService.getUserRole({ allowCachedOnError: false });
			if (userRole === 'INSTANCE_ADMIN') {
				return true;
			}
			this.router.navigate(['/projects']);
			return false;
		} catch (error) {
			console.error('Error checking instance admin access:', error);
			this.router.navigate(['/projects']);
			return false;
		}
	}
}
