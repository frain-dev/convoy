import { Component, Input, OnInit } from '@angular/core';


@Component({
    selector: '[convoy-card]',
    imports: [],
    host: { class: 'rounded-8px', '[class]': 'classes' },
    template: `
		<ng-content></ng-content>
	`
})
export class CardComponent implements OnInit {
	@Input('hover') hover: 'true' | 'false' = 'false';
	@Input('color') color: 'default' | 'error' = 'default';

	constructor() {}

	ngOnInit(): void {}

	get classes(): string {
		const colors = {
			default: 'bg-white-100 border border-new.border',
			error: 'bg-error-a3 border border-error-6'
		};
		return `${colors[this.color]} ${this.hover === 'true' ? 'hover:bg-new.surface-subtle hover:border-new.border focus:border-new.border focus-visible:border-new.border outline-none transition-all duration-300' : ''} block`;
	}
}
