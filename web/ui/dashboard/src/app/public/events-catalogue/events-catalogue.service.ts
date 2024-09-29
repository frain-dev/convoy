import { Injectable } from '@angular/core';
// import SwaggerParser from '@apidevtools/swagger-parser';
// import sampler from 'openapi-sampler';
// @ts-ignore
// import yaml from 'js-yaml';
import { HttpService } from 'src/app/services/http/http.service';
import { HTTP_RESPONSE } from 'src/app/models/global.model';
// @ts-ignore
import gs from 'generate-schema';

@Injectable({
	providedIn: 'root'
})
export class EventsCatalogueService {
	constructor(private http: HttpService) {}

	getEventCatlogue(): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				const response = await this.http.request({ url: `/view_event_catalogue`, method: 'get' });
				return resolve(response);
			} catch (error) {
				return reject(error);
			}
		});
	}

	// async processOpenAPI(openApiFilePath: any) {
	// 	const sample = sampler.sample;

	// 	try {
	// 		const dereferencedData = await SwaggerParser.dereference(yaml.load(openApiFilePath));
	// 		var final: any = [];

	// 		Object.entries(dereferencedData.webhooks).forEach(([key, value]: [string, any]) => {
	// 			let schema = structuredClone(value.post.requestBody.content['application/json'].schema);

	// 			schema['description'] = value.post.requestBody.description;
	// 			schema['sample_json'] = sample(schema);
	// 			schema['name'] = key;

	// 			delete schema.required;
	// 			final.push(schema);
	// 		});

	// 		return final;
	// 	} catch (error) {
	// 		console.error('Error processing OpenAPI spec:', error);
	// 		return null;
	// 	}
	// }

	async processJSONEvent(events: any) {
		try {
			var final: any = [];

			events.forEach((event: any) => {
				const schema = gs.json('Event', event.Data);

				let properties: any = this.processObject(event.Data, schema.properties);

				final.push({ name: event.Name, sample_json: event.Data, properties });
			});
			return final;
		} catch (error) {
			console.error('Error processing JSON events:', error);
			return null;
		}
	}

	processObject(eventData: any, schemaProperty: any) {
		let properties: { name: string; type: string; value?: any; children?: any }[] = [];
		let children: { name: string; type: string; value?: any; children?: any }[] = [];

		Object.entries(eventData).forEach(([key, value]) => {
			Object.entries(schemaProperty).forEach(([schemaKey, schemaValue]: [string, any]) => {
				if (key === schemaKey) {
					if (schemaValue.type === 'object' && typeof value === 'object' && value !== null) {
						children = this.processObject(value, schemaValue.properties);
						properties.push({ name: schemaKey, type: schemaValue.type, children });
					} else properties.push({ name: schemaKey, type: schemaValue.type, value });
				}
			});
		});

		return properties;
	}
}
