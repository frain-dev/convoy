import { Pipe, PipeTransform } from '@angular/core';

@Pipe({
	name: 'sourceValue'
})
export class SourceValuePipe implements PipeTransform {
	sourceTypes = [
		{ value: 'http', viewValue: 'HTTP' },
		{ value: 'rest_api', viewValue: 'Rest API' },
		{ value: 'pub_sub', viewValue: 'Pub/Sub' },
		{ value: 'db_change_stream', viewValue: 'Database' }
	];
	httpTypes = [
		{ value: 'hmac', viewValue: 'HMAC' },
		{ value: 'basic_auth', viewValue: 'Basic Auth' },
		{ value: 'api_key', viewValue: 'API Key' },
		{ value: 'noop', viewValue: 'None' }
	];

	transform(value: string, type: 'sourceType' | 'verifier'): string {
		if (type === 'sourceType') {
			return this.sourceTypes.find(source => source.value === value)?.viewValue || '-';
		}
		if (type === 'verifier') {
			return this.httpTypes.find(source => source.value === value)?.viewValue || '-';
		}

		return '-';
	}
}
