import { Component, Input, OnInit } from '@angular/core';
import { CommonModule } from '@angular/common';

@Component({
	selector: 'convoy-card',
	standalone: true,
	imports: [CommonModule],
	templateUrl: './card.component.html',
	styleUrls: ['./card.component.scss']
})
export class CardComponent implements OnInit {
	@Input('className') class!: string;
	@Input('color') color: 'default' | 'danger' = 'default';

	constructor() {}

	ngOnInit(): void {}

	get classes(): string {
		const colors = {
			default: 'bg-white-100 border-grey-10',
			danger: 'bg-danger-500 border-danger-200'
		};
		return `${colors[this.color]} ${this.class}`;
	}
}
