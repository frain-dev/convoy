import { Pipe, PipeTransform } from '@angular/core';
import { STATUS_COLOR } from '../../models/global.model';

@Pipe({
	name: 'statuscolor'
})
export class StatusColorPipe implements PipeTransform {
	transform(value: string): STATUS_COLOR {
		let type: STATUS_COLOR = 'grey';

		switch (value) {
			case 'default':
			case 'offline':
				type = 'grey';
				break;
			case 'active':
			case 'Success':
			case 'success':
			case 'online':
				type = 'success';
				break;
			case 'Pending':
				type = 'warning';
				break;
			case 'Failed':
			case 'Failure':
			case 'disabled':
				type = 'danger';
				break;

			default:
				break;
		}
		return type;
	}
}
