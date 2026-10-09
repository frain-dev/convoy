import { Pipe, PipeTransform } from '@angular/core';

@Pipe({
	name: 'countLabel',
	standalone: true
})
export class CountLabelPipe implements PipeTransform {
	transform(count: number | null | undefined, singular: string, plural: string): string {
		return count === 1 ? singular : plural;
	}
}
