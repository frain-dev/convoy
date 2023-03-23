import { Component, Input, OnInit } from '@angular/core';
import { CommonModule } from '@angular/common';

@Component({
	selector: '[convoy-card]',
	standalone: true,
	imports: [CommonModule],
	host: { class: 'rounded-8px', '[class]': 'classes' },
	template: `
		<ng-content></ng-content>
	`
})
export class CardComponent implements OnInit {
	@Input('hover') hover: 'true' | 'false' = 'false';
	@Input('color') color: 'default' | 'danger' = 'default';

	constructor() {}

	ngOnInit(): void {}

	get classes(): string {
		const colors = {
			default: 'bg-[#fff] border border-grey-10',
			danger: 'bg-danger-500 border border-danger-200'
		};
		return `${colors[this.color]} ${this.hover === 'true' ? 'focus:shadow-sm hover:shadow-sm focus-visible:shadow-sm hover:border-grey-20 focus:border-grey-20 focus-visible:border-grey-20 outline-none' : ''} block`;
	}
}
