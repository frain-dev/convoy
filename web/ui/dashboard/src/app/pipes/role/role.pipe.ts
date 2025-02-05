import { Pipe, PipeTransform } from '@angular/core';

@Pipe({
	name: 'role',
	standalone: true
})
export class RolePipe implements PipeTransform {
	transform(value: string): string {
		switch (value) {
            case 'root':
                return 'Root'
            case 'instance_admin':
                return 'Instance Admin';
			case 'organisation_admin':
				return 'Organisation Admin';
			case 'admin':
				return 'Admin';
			case 'member':
				return 'Member';
			default:
				return '-';
		}
	}
}
