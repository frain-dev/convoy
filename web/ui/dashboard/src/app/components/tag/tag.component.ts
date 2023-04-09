import { Component, Input, OnInit } from '@angular/core';
import { CommonModule } from '@angular/common';
import { STATUS_COLOR } from 'src/app/models/global.model';

@Component({
	selector: 'convoy-tag, [convoy-tag]',
	standalone: true,
	imports: [CommonModule],
	template: `
		<ng-content></ng-content>
	`,
	styleUrls: ['./tag.component.scss'],
	host: { class: 'py-[1px] px-8px rounded-8px w-fit text-center text-12 font-medium', '[class]': 'classes' }
})
export class TagComponent implements OnInit {
	@Input('type') type: STATUS_COLOR = 'grey';
	@Input('className') class!: string;

	constructor() {}

	ngOnInit(): void {}

	get classes() {
		if (this.type === 'grey') return `text-grey-40 bg-grey-10 ${this.class}`;
		return ` text-${this.type}-100 bg-${this.type}-500  ${this.class}`;
	}
}
