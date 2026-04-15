import {Injectable} from '@angular/core';
import {ActivatedRoute} from '@angular/router';
import {PrivateService} from 'src/app/private/private.service';

@Injectable({
	providedIn: 'root'
})
export class RbacService {
	private static readonly lastKnownRoleStorageKeyPrefix = 'CONVOY_LAST_USER_ROLE_';

	permissions = {
		PROJECT_VIEWER: ['Event Deliveries|VIEW', 'Sources|VIEW', 'Subscriptions|VIEW', 'Endpoints|VIEW', 'Portal Links|VIEW', 'Events|VIEW', 'Meta Events|VIEW', 'Project Settings|VIEW', 'Projects|VIEW', 'Team|VIEW', 'Organisations|VIEW', 'Organisations|ADD'],
		PROJECT_ADMIN: ['Event Deliveries|MANAGE', 'Sources|MANAGE', 'Subscriptions|MANAGE', 'Endpoints|MANAGE', 'Portal Links|MANAGE', 'Events|MANAGE', 'Meta Events|MANAGE', 'Project Settings|MANAGE', 'Projects|MANAGE', 'Event Types|MANAGE', 'Project Setup|MANAGE', 'Organisations|ADD'],
		ORGANISATION_ADMIN: ['Team|MANAGE', 'Organisations|MANAGE', 'Project Setup|MANAGE', 'Organisations|ADD'],
		BILLING_ADMIN: ['Billing|MANAGE', 'Organisations|ADD'],
		INSTANCE_ADMIN: ['Instance|MANAGE', 'Project Setup|MANAGE', 'Organisations|ADD']
	};

	constructor(private privateService: PrivateService, private route: ActivatedRoute) {}

	async getUserRole(options?: { allowCachedOnError?: boolean }): Promise<ROLE> {
		const allowCachedOnError = options?.allowCachedOnError ?? true;
		try {
			const member = await this.privateService.getOrganizationMembership({ refresh: true });
			const role = member.data.content[0].role.type as string | undefined;
			const mappedRole = this.mapRole(role);
			this.persistLastKnownRole(mappedRole);
			return mappedRole;
		} catch (error) {
			if (allowCachedOnError) {
				const cachedRole = this.getLastKnownRole();
				// Never recover admin access from cache on fetch failures.
				if (cachedRole && cachedRole !== 'INSTANCE_ADMIN') return cachedRole;
			}
			return 'PROJECT_VIEWER';
		}
	}

	private mapRole(role: string | undefined): ROLE {
		switch (role) {
			case 'instance_admin':
				return 'INSTANCE_ADMIN';
			case 'organisation_admin':
				return 'ORGANISATION_ADMIN';
			case 'billing_admin':
				return 'BILLING_ADMIN';
			case 'project_admin':
				return 'PROJECT_ADMIN';
			default:
				return 'PROJECT_VIEWER';
		}
	}

	private persistLastKnownRole(role: ROLE): void {
		try {
			const userId = this.getCurrentUserId();
			if (!userId) return;
			localStorage.setItem(`${RbacService.lastKnownRoleStorageKeyPrefix}${userId}`, role);
		} catch {}
	}

	private getLastKnownRole(): ROLE | null {
		try {
			const userId = this.getCurrentUserId();
			if (!userId) return null;
			const cachedRole = localStorage.getItem(`${RbacService.lastKnownRoleStorageKeyPrefix}${userId}`);
			switch (cachedRole) {
				case 'INSTANCE_ADMIN':
				case 'ORGANISATION_ADMIN':
				case 'BILLING_ADMIN':
				case 'PROJECT_ADMIN':
				case 'PROJECT_VIEWER':
					return cachedRole;
				default:
					return null;
			}
		} catch {
			return null;
		}
	}

	private getCurrentUserId(): string | null {
		try {
			const authData = localStorage.getItem('CONVOY_AUTH');
			const auth = authData ? JSON.parse(authData) : null;
			return auth?.uid ?? null;
		} catch {
			return null;
		}
	}

	public async userCanAccess(requestPermission: PERMISSION): Promise<boolean> {
		const permissions = await this.userPermission();
		const portalPermissions = ['Subscriptions|MANAGE', 'Endpoints|MANAGE'];
		if (this.route.snapshot.queryParams['token']) return !!portalPermissions.find(permission => permission == requestPermission);
		return !!permissions.find(permission => permission == requestPermission);
	}

	async userPermission(): Promise<string[]> {
		const role = await this.getUserRole();

		let permissions;
		switch (role) {
			case 'INSTANCE_ADMIN':
				permissions = this.permissions[role].concat(
					this.permissions.ORGANISATION_ADMIN,
					this.permissions.BILLING_ADMIN,
					this.permissions.PROJECT_ADMIN,
					this.permissions.PROJECT_VIEWER
				);
				break;
			case 'ORGANISATION_ADMIN':
				permissions = this.permissions[role].concat(
					this.permissions.PROJECT_ADMIN,
					this.permissions.PROJECT_VIEWER
				);
				break;
			case 'BILLING_ADMIN':
				permissions = this.permissions[role];
				break;
			case 'PROJECT_ADMIN':
				permissions = this.permissions[role].concat(this.permissions.PROJECT_VIEWER);
				break;
			default:
				permissions = this.permissions.PROJECT_VIEWER;
				break;
		}

		return permissions;
	}
}

export type PERMISSION =
	| 'Event Deliveries|VIEW'
	| 'Event Deliveries|MANAGE'
	| 'Sources|VIEW'
	| 'Sources|MANAGE'
	| 'Subscriptions|VIEW'
	| 'Subscriptions|MANAGE'
	| 'Endpoints|VIEW'
	| 'Endpoints|MANAGE'
	| 'Portal Links|VIEW'
	| 'Portal Links|MANAGE'
	| 'Events|VIEW'
	| 'Events|MANAGE'
	| 'Meta Events|VIEW'
	| 'Meta Events|MANAGE'
	| 'Project Settings|VIEW'
	| 'Project Settings|MANAGE'
	| 'Projects|VIEW'
	| 'Projects|MANAGE'
	| 'Team|VIEW'
	| 'Team|MANAGE'
	| 'Organisations|VIEW'
	| 'Organisations|MANAGE'
	| 'Organisations|ADD'
	| 'Billing|MANAGE'
	| 'Instance|MANAGE'
	| 'Event Types|MANAGE'
	| 'Project Setup|MANAGE';

export type ROLE = 'PROJECT_VIEWER' | 'PROJECT_ADMIN' | 'ORGANISATION_ADMIN' | 'BILLING_ADMIN' | 'INSTANCE_ADMIN';
