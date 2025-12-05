import { Injectable } from '@angular/core';
import { CanActivate, Router } from '@angular/router';
import { RbacService } from '../services/rbac/rbac.service';

@Injectable({
	providedIn: 'root'
})
export class InstanceAdminGuard implements CanActivate {
	constructor(
		private rbacService: RbacService,
		private router: Router
	) {}

	async canActivate(): Promise<boolean> {
		try {
			const userRole = await this.rbacService.getUserRole();
			if (userRole === 'INSTANCE_ADMIN') {
				return true;
			}
			// Redirect to projects if not instance admin
			this.router.navigate(['/projects']);
			return false;
		} catch (error) {
			console.error('Error checking instance admin access:', error);
			this.router.navigate(['/projects']);
			return false;
		}
	}
}
