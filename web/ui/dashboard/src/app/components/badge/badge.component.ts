import { CommonModule } from '@angular/common';
import { Component, Input, OnInit } from '@angular/core';

@Component({
	selector: 'convoy-badge',
	standalone: true,
	imports: [CommonModule],
	templateUrl: './badge.component.html',
	styleUrls: ['./badge.component.scss']
})
export class BadgeComponent implements OnInit {
	@Input('color') color!: string;
	@Input('text') text!: string;
	constructor() {}

	ngOnInit(): void {}

	get firstletters(): string {
		const firstLetters = this.text
			.split(' ')
			.map(word => word[0])
			.join('');
		return firstLetters;
	}
}
