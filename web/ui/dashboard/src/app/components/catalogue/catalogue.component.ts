import { Component, Input, OnInit } from '@angular/core';
import { CommonModule } from '@angular/common';
import { TagComponent } from '../tag/tag.component';

@Component({
	selector: 'convoy-catalogue',
	standalone: true,
	imports: [CommonModule, TagComponent],
	templateUrl: './catalogue.component.html',
	styleUrls: ['./catalogue.component.scss']
})
export class CatalogueComponent implements OnInit {
	@Input('catalog') catalog: any;
	selectedProperty!: string;
	constructor() {}

	ngOnInit(): void {}

	collapseCatalog(name: string) {
		this.selectedProperty = name;
	}

	getVariableType(type: string): { type: string; color: 'error' | 'primary' | 'warning' | 'neutral' | 'success'; fill: 'outline' | 'soft' | 'solid' | 'soft-outline' } {
		let varObj: { type: string; color: 'error' | 'primary' | 'warning' | 'neutral' | 'success'; fill: 'outline' | 'soft' | 'solid' | 'soft-outline' };
		switch (type) {
			case 'string':
				varObj = {
					type: 'string',
					color: 'primary',
					fill: 'soft'
				};
				break;
			case 'number':
				varObj = {
					type: 'number',
					color: 'success',
					fill: 'soft'
				};
				break;
			case 'boolean':
				varObj = {
					type: 'boolean',
					color: 'neutral',
					fill: 'soft'
				};
				break;
			case 'array':
				varObj = {
					type: 'array[]',
					color: 'warning',
					fill: 'soft'
				};
				break;
			case 'object':
				varObj = {
					type: 'object',
					color: 'error',
					fill: 'soft'
				};
				break;
			case 'null':
				varObj = {
					type: 'null',
					color: 'neutral',
					fill: 'soft'
				};
				break;
			default:
				varObj = {
					type,
					color: 'neutral',
					fill: 'soft'
				};
				break;
		}
		return varObj;
	}
}
