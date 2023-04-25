import { Injectable } from '@angular/core';

@Injectable({
	providedIn: 'root'
})
export class RbacService {
	permissions = {
		MEMBER: ['Event Deliveries|VIEW', 'Event Deliveries|MANAGE', 'Sources|VIEW', 'Subscriptions|VIEW', 'Endpoints|VIEW', 'Portal Links|VIEW', 'Events|VIEW', 'Events|MANAGE', 'Meta Events|VIEW', 'Project Settings|VIEW', 'Projects|VIEW', 'Team|VIEW', 'Organisations: VIEW'],
		SUPER_ADMIN: ['Team|MANAGE', 'Organisations: MANAGE'],
		ADMIN: ['Sources|MANAGE', 'Subscriptions|MANAGE', 'Endpoints|MANAGE', 'Portal Links|MANAGE', 'Meta Events|MANAGE', 'Project Settings|MANAGE', 'Projects|MANAGE']
	};

	constructor() {}

	private get getUserRole(): ROLE {
		return 'MEMBER';
	}

	public userCanAccess(requestPermission: PERMISSION): boolean {
		return !!this.userPermission.find(permission => permission == requestPermission);
	}

	public get userPermission(): string[] {
		const role = this.getUserRole;

		let permissions;
		switch (role) {
			case 'SUPER_ADMIN':
				permissions = this.permissions[role].concat(this.permissions.ADMIN, this.permissions.MEMBER);
				break;
			case 'ADMIN':
				permissions = this.permissions[role].concat(this.permissions.MEMBER);
				break;
			default:
				permissions = this.permissions.MEMBER;
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
	| 'Organisations|MANAGE';

export type ROLE = 'MEMBER' | 'ADMIN' | 'SUPER_ADMIN';
