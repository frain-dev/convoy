import { Component, Input, OnInit } from '@angular/core';
import { CommonModule } from '@angular/common';
import { STATUS_COLOR } from 'src/app/models/global.model';

@Component({
	selector: 'convoy-tag, [convoy-tag]',
	standalone: true,
	imports: [CommonModule],
	templateUrl: './tag.component.html',
	styleUrls: ['./tag.component.scss']
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
