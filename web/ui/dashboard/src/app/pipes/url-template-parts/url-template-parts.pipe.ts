import { Pipe, PipeTransform } from '@angular/core';

export interface UrlTemplatePart {
	text: string;
	token: boolean;
}

// Splits a URL into segments so templated tokens like {reference} can be rendered
// distinctly from the static parts of the URL. Token names follow the backend
// endpoint URL template rules: {name} where name is [A-Za-z_][A-Za-z0-9_]*.
@Pipe({
	name: 'urlTemplateParts',
	standalone: true
})
export class UrlTemplatePartsPipe implements PipeTransform {
	transform(value?: string | null): UrlTemplatePart[] {
		if (!value) return [];
		return value
			.split(/(\{[A-Za-z_][A-Za-z0-9_]*\})/g)
			.filter(part => part !== '')
			.map(part => ({ text: part, token: /^\{[A-Za-z_][A-Za-z0-9_]*\}$/.test(part) }));
	}
}
