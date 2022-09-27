import { Pipe, PipeTransform } from '@angular/core';

@Pipe({
	name: 'formatSeconds',
	standalone: true
})
export class FormatSecondsPipe implements PipeTransform {

	transform(timeValue?: number): unknown {
		if (timeValue && timeValue >= 60) {
			const timeInMinutes = Math.floor(timeValue / 60);
			const remainderSeconds = timeValue % 60;
			return `${timeInMinutes}m${remainderSeconds ? `${remainderSeconds}s` : ''}`;
		}

		return `${timeValue ? timeValue : 0}s`;
	}
}
