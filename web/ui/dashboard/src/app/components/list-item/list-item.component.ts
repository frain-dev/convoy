import { Component, Input, OnInit } from '@angular/core';


@Component({
    selector: 'convoy-list-item, [convoy-list-item]',
    host: { class: 'flex items-center justify-between py-10px transition-all duration-300 hover:bg-new.surface-muted', '[class]': 'class' },
    imports: [],
    template: `
		<ng-content></ng-content>
	`
})
export class ListItemComponent implements OnInit {
	@Input('hasBorder') hasBorder = true;
	@Input('active') active: 'true' | 'false' = 'false';

	constructor() {}

	ngOnInit(): void {}

	get class() {
		return `${this.hasBorder ? 'border-new.border border-b' : ''} ${this.active === 'true' ? 'bg-new.surface-muted' : ''}`;
	}
}
