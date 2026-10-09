import {Pipe, PipeTransform} from '@angular/core';
import {STATUS_COLOR} from '../../models/global.model';

@Pipe({
    name: 'statuscolor',
    standalone: false
})
export class StatusColorPipe implements PipeTransform {
	transform(value: string): STATUS_COLOR {
		let type: STATUS_COLOR = 'neutral';

		switch (value) {
			case 'default':
			case 'offline':
				type = 'neutral';
				break;
			case 'active':
			case 'Success':
			case 'success':
			case 'online':
			case 'Paid':
			case 'paid':
			case 'completed':
				type = 'success';
				break;
			case 'Pending':
			case 'pending':
			case 'running':
				type = 'warning';
				break;
			case 'Failed':
			case 'Failure':
			case 'failed':
			case 'disabled':
			case 'Overdue':
			case 'overdue':
				type = 'error';
				break;
			case 'Discarded':
			case 'discarded':
			case 'Scheduled':
			case 'Processing':
			case 'Retry':
				type = 'neutral';
				break;

			default:
				break;
		}
		return type;
	}
}
