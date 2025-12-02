import {Pipe, PipeTransform} from '@angular/core';

@Pipe({
	name: 'role',
	standalone: true
})
export class RolePipe implements PipeTransform {
	transform(value: string): string {
		switch (value) {
			case 'instance_admin':
				return 'Instance Admin';
			case 'organisation_admin':
				return 'Organisation Admin';
			case 'billing_admin':
				return 'Billing Admin';
			case 'project_admin':
				return 'Project Admin';
			case 'project_viewer':
				return 'Project Viewer';
			default:
				return '-';
		}
	}
}
