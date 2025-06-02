import { Injectable } from '@angular/core';
import { ActivatedRoute } from '@angular/router';
import { PrivateService } from 'src/app/private/private.service';

@Injectable({
	providedIn: 'root'
})
export class RbacService {
	permissions = {
		PROJECT_VIEWER: ['Event Deliveries|VIEW', 'Sources|VIEW', 'Subscriptions|VIEW', 'Endpoints|VIEW', 'Portal Links|VIEW', 'Events|VIEW', 'Meta Events|VIEW', 'Project Settings|VIEW', 'Projects|VIEW', 'Team|VIEW', 'Organisations|VIEW'],
		PROJECT_ADMIN: ['Event Deliveries|MANAGE', 'Sources|MANAGE', 'Subscriptions|MANAGE', 'Endpoints|MANAGE', 'Portal Links|MANAGE', 'Events|MANAGE', 'Meta Events|MANAGE', 'Project Settings|MANAGE', 'Projects|MANAGE', 'Event Types|MANAGE'],
		ORGANISATION_ADMIN: ['Team|MANAGE', 'Organisations|MANAGE'],
		BILLING_ADMIN: ['Billing|MANAGE'],
		INSTANCE_ADMIN: ['Instance|MANAGE']
	};

	constructor(private privateService: PrivateService, private route: ActivatedRoute) {}

	async getUserRole(): Promise<ROLE> {
		try {
			const member = await this.privateService.getOrganizationMembership();
			const role = member.data.content[0].role.type;
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
		} catch (error) {
			return 'PROJECT_VIEWER';
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
	| 'Billing|MANAGE'
	| 'Instance|MANAGE'
	| 'Event Types|MANAGE';

export type ROLE = 'PROJECT_VIEWER' | 'PROJECT_ADMIN' | 'ORGANISATION_ADMIN' | 'BILLING_ADMIN' | 'INSTANCE_ADMIN';
