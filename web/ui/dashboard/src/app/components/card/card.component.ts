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
			default: 'bg-white-100 border border-neutral-4',
			danger: 'bg-danger-500 border border-danger-200'
		};
		return `${colors[this.color]} ${this.hover === 'true' ? 'focus:shadow-default hover:shadow-default focus-visible:shadow-default hover:border-neutral-4 focus:border-neutral-4 focus-visible:border-neutral-4 outline-none transition-all duration-300' : ''} block`;
	}
}
