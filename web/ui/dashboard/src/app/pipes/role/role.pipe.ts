import { Pipe, PipeTransform } from '@angular/core';

@Pipe({
	name: 'role',
	standalone: true
})
export class RolePipe implements PipeTransform {
	transform(value: string): string {
		switch (value) {
			case 'super_user':
				return 'Super User';
			case 'admin':
				return 'Admin';
			case 'member':
				return 'Member';
			default:
				return '-';
		}
	}
}
