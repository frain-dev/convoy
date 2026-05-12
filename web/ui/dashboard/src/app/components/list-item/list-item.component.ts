import { Component, Input, OnInit } from '@angular/core';


@Component({
    selector: 'convoy-list-item, [convoy-list-item]',
    host: { class: 'flex items-center justify-between py-10px transition-all duration-300 hover:bg-primary-500', '[class]': 'class' },
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
		return `${this.hasBorder ? 'border-neutral-a3 border-b' : ''} ${this.active === 'true' ? 'bg-primary-500' : ''}`;
	}
}
